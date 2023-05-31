package butterfish

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/bakks/butterfish/prompt"
	"github.com/bakks/butterfish/util"

	"github.com/mitchellh/go-ps"
	"golang.org/x/term"
)

const ESC_CUP = "\x1b[6n" // Request the cursor position
const ESC_UP = "\x1b[%dA"
const ESC_RIGHT = "\x1b[%dC"
const ESC_LEFT = "\x1b[%dD"
const ESC_CLEAR = "\x1b[0K"

var DarkShellColorScheme = &ShellColorScheme{
	Prompt:       "\x1b[38;5;154m",
	PromptAction: "\x1b[38;5;200m",
	Command:      "\x1b[0m",
	Autosuggest:  "\x1b[38;5;241m",
	Answer:       "\x1b[38;5;214m",
	Aquarium:     "\x1b[38;5;51m",
	Error:        "\x1b[38;5;196m",
}

var LightShellColorScheme = &ShellColorScheme{
	Prompt:       "\x1b[38;5;28m",
	PromptAction: "\x1b[38;5;200m",
	Command:      "\x1b[0m",
	Autosuggest:  "\x1b[38;5;241m",
	Answer:       "\x1b[38;5;214m",
	Aquarium:     "\x1b[38;5;18m",
	Error:        "\x1b[38;5;196m",
}

func RunShell(ctx context.Context, config *ButterfishConfig) error {
	envVars := []string{"BUTTERFISH_SHELL=1"}

	ptmx, ptyCleanup, err := ptyCommand(ctx, envVars, []string{config.ShellBinary})
	if err != nil {
		return err
	}
	defer ptyCleanup()

	bf, err := NewButterfish(ctx, config)
	if err != nil {
		return err
	}
	//fmt.Println("Starting butterfish shell")

	bf.ShellMultiplexer(ptmx, ptmx, os.Stdin, os.Stdout)
	return nil
}

const (
	historyTypePrompt = iota
	historyTypeShellInput
	historyTypeShellOutput
	historyTypeLLMOutput
)

// Turn history type enum to a string
func HistoryTypeToString(historyType int) string {
	switch historyType {
	case historyTypePrompt:
		return "Prompt"
	case historyTypeShellInput:
		return "Shell Input"
	case historyTypeShellOutput:
		return "Shell Output"
	case historyTypeLLMOutput:
		return "LLM Output"
	default:
		return "Unknown"
	}
}

type HistoryBuffer struct {
	Type    int
	Content *ShellBuffer
}

// ShellHistory keeps a record of past shell history and LLM interaction in
// a slice of util.HistoryBlock objects. You can add a new block, append to
// the last block, and get the the last n bytes of the history as an array of
// HistoryBlocks.
type ShellHistory struct {
	Blocks []HistoryBuffer
}

func NewShellHistory() *ShellHistory {
	return &ShellHistory{
		Blocks: make([]HistoryBuffer, 0),
	}
}

func (this *ShellHistory) add(historyType int, block string) {
	buffer := NewShellBuffer()
	buffer.Write(block)
	this.Blocks = append(this.Blocks, HistoryBuffer{
		Type:    historyType,
		Content: buffer,
	})
}

func (this *ShellHistory) Append(historyType int, data string) {
	// if data is empty, we don't want to add a new block
	if len(data) == 0 {
		return
	}

	numBlocks := len(this.Blocks)
	// if we have a block already, and it matches the type, append to it
	if numBlocks > 0 {
		lastBlock := this.Blocks[numBlocks-1]

		if lastBlock.Type == historyType {
			lastBlock.Content.Write(data)
			return
		}
	}

	// if the history type doesn't match we fall through and add a new block
	this.add(historyType, data)
}

func (this *ShellHistory) NewBlock() {
	length := len(this.Blocks)
	if length > 0 {
		this.add(this.Blocks[length-1].Type, "")
	}
}

// Go back in history for a certain number of bytes.
func (this *ShellHistory) GetLastNBytes(numBytes int, truncateLength int) []util.HistoryBlock {
	var blocks []util.HistoryBlock

	for i := len(this.Blocks) - 1; i >= 0 && numBytes > 0; i-- {
		block := this.Blocks[i]
		content := sanitizeTTYString(block.Content.String())
		if len(content) > truncateLength {
			content = content[:truncateLength]
		}
		if len(content) > numBytes {
			break // we don't want a weird partial line so we bail out here
		}
		blocks = append(blocks, util.HistoryBlock{
			Type:    block.Type,
			Content: content,
		})
		numBytes -= len(content)
	}

	// reverse the blocks slice
	for i := len(blocks)/2 - 1; i >= 0; i-- {
		opp := len(blocks) - 1 - i
		blocks[i], blocks[opp] = blocks[opp], blocks[i]
	}

	return blocks
}

func (this *ShellHistory) LogRecentHistory() {
	blocks := this.GetLastNBytes(2000, 512)
	log.Printf("Recent history: =======================================")
	builder := strings.Builder{}
	for _, block := range blocks {
		builder.WriteString(fmt.Sprintf("%s: %s\n", HistoryTypeToString(block.Type), block.Content))
	}
	log.Printf(builder.String())
	log.Printf("=======================================")
}

func HistoryBlocksToString(blocks []util.HistoryBlock) string {
	var sb strings.Builder
	for i, block := range blocks {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(block.Content)
	}
	return sb.String()
}

const (
	stateNormal = iota
	stateShell
	statePrompting
	statePromptResponse
)

var stateNames = []string{
	"Normal",
	"Shell",
	"Prompting",
	"PromptResponse",
}

type AutosuggestResult struct {
	Command    string
	Suggestion string
}

type ShellColorScheme struct {
	Prompt       string
	PromptAction string
	Error        string
	Command      string
	Autosuggest  string
	Answer       string
	Aquarium     string
}

type ShellState struct {
	Butterfish *ButterfishCtx
	ParentOut  io.Writer
	ChildIn    io.Writer
	Sigwinch   chan os.Signal

	// The current state of the shell
	State                int
	AquariumMode         bool
	AquariumBuffer       string
	PromptSuffixCounter  int
	ChildOutReader       chan *byteMsg
	ParentInReader       chan *byteMsg
	CursorPosChan        chan *cursorPosition
	PromptOutputChan     chan *byteMsg
	AutosuggestChan      chan *AutosuggestResult
	History              *ShellHistory
	PromptAnswerWriter   io.Writer
	Prompt               *ShellBuffer
	PromptResponseCancel context.CancelFunc
	Command              *ShellBuffer
	TerminalWidth        int
	Color                *ShellColorScheme

	// autosuggest config
	AutosuggestEnabled bool
	LastAutosuggest    string
	AutosuggestCtx     context.Context
	AutosuggestCancel  context.CancelFunc
	AutosuggestBuffer  *ShellBuffer
}

func (this *ShellState) setState(state int) {
	log.Printf("State change: %s -> %s", stateNames[this.State], stateNames[state])
	this.State = state
}

func clearByteChan(r <-chan *byteMsg, timeout time.Duration) {
	for {
		select {
		case <-time.After(timeout):
			return
		case <-r:
			continue
		}
	}
}

func (this *ShellState) GetCursorPosition() (int, int) {
	// send the cursor position request
	this.ParentOut.Write([]byte(ESC_CUP))
	timeout := time.After(100 * time.Millisecond)
	var pos *cursorPosition

	// the parent in reader watches for these responses, set timeout and
	// panic if we don't get a response
	select {
	case <-timeout:
		panic("Timeout waiting for cursor position response, this probably means that you're using a terminal emulator that doesn't work well with butterfish. Please submit an issue to https://github.com/bakks/butterfish.")

	case pos = <-this.CursorPosChan:
	}

	// it's possible that we have a stale response, so we loop on the channel
	// until we get the most recent one
	for {
		select {
		case pos = <-this.CursorPosChan:
			continue
		default:
			return pos.Row, pos.Column
		}
	}
}

// Special characters that we wrap the shell's command prompt in (PS1) so
// that we can detect where it starts and ends.
const promptPrefix = "\033Q"
const promptSuffix = "\033R"
const promptPrefixEscaped = "\\033Q"
const promptSuffixEscaped = "\\033R"

var ps1Regex = regexp.MustCompile(" ([0-9]+)" + promptSuffix)

// This sets the PS1 shell variable, which is the prompt that the shell
// displays before each command.
// We need to be able to parse the child shell's prompt to determine where
// it starts, ends, exit code, and allow customization to show the user that
// we're inside butterfish shell. The PS1 is roughly the following:
// PS1 := promptPrefix $PS1 ShellCommandPrompt $? promptSuffix
func (this *ButterfishCtx) SetPS1(childIn io.Writer) {
	shell := this.Config.ParseShell()
	var ps1 string

	switch shell {
	case "bash", "sh":
		// the \[ and \] are bash-specific and tell bash to not count the enclosed
		// characters when calculating the cursor position
		ps1 = "PS1=$'\\[%s\\]'$PS1$'%s\\[ $?%s\\]'\n"
	case "zsh":
		// the %%{ and %%} are zsh-specific and tell zsh to not count the enclosed
		// characters when calculating the cursor position
		ps1 = "PS1=$'%%{%s%%}'$PS1$'%s%%{ %%?%s%%}'\n"
	default:
		log.Printf("Unknown shell %s, Butterfish is going to leave the PS1 alone. This means that you won't get a custom prompt in Butterfish, and Butterfish won't be able to parse the exit code of the previous command, used for centain features. Create an issue at https://github.com/bakks/butterfish.", shell)
		return
	}

	fmt.Fprintf(childIn,
		ps1,
		promptPrefixEscaped,
		this.Config.ShellCommandPrompt,
		promptSuffixEscaped)
}

// Given a string of terminal output, identify terminal prompts based on the
// custom PS1 escape sequences we set.
// Returns:
// - The last exit code/status seen in the string (i.e. will be non-zero if
//   previous command failed.
// - The number of prompts identified in the string.
// - The string with the special prompt escape sequences removed.
func ParsePS1(data string) (int, int, string) {
	matches := ps1Regex.FindAllStringSubmatch(data, -1)
	lastStatus := 0
	prompts := 0

	for _, match := range matches {
		var err error
		lastStatus, err = strconv.Atoi(match[1])
		if err != nil {
			log.Printf("Error parsing PS1 match: %s", err)
		}
		prompts++
	}
	// Remove matches of suffix
	cleaned := ps1Regex.ReplaceAllString(data, "")
	// Remove the prefix
	cleaned = strings.ReplaceAll(cleaned, promptPrefix, "")

	return lastStatus, prompts, cleaned
}

func (this *ButterfishCtx) ShellMultiplexer(
	childIn io.Writer, childOut io.Reader,
	parentIn io.Reader, parentOut io.Writer) {

	this.SetPS1(childIn)

	colorScheme := DarkShellColorScheme
	if !this.Config.ShellColorDark {
		colorScheme = LightShellColorScheme
	}

	log.Printf("Starting shell multiplexer")

	childOutReader := make(chan *byteMsg)
	parentInReader := make(chan *byteMsg)
	parentPositionChan := make(chan *cursorPosition)

	go readerToChannel(childOut, childOutReader)
	go readerToChannelWithPosition(parentIn, parentInReader, parentPositionChan)

	carriageReturnWriter := util.NewReplaceWriter(parentOut, "\n", "\r\n")

	termWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		panic(err)
	}

	sigwinch := make(chan os.Signal, 1)
	signal.Notify(sigwinch, syscall.SIGWINCH)

	//	if this.Config.ShellPluginMode {
	//		client, err := this.StartPluginClient()
	//		if err != nil {
	//			panic(err)
	//		}
	//
	//		go client.Mux(this.Ctx)
	//	}

	shellState := &ShellState{
		Butterfish:         this,
		ParentOut:          parentOut,
		ChildIn:            childIn,
		Sigwinch:           sigwinch,
		State:              stateNormal,
		ChildOutReader:     childOutReader,
		ParentInReader:     parentInReader,
		CursorPosChan:      parentPositionChan,
		History:            NewShellHistory(),
		PromptOutputChan:   make(chan *byteMsg),
		PromptAnswerWriter: carriageReturnWriter,
		Command:            NewShellBuffer(),
		Prompt:             NewShellBuffer(),
		TerminalWidth:      termWidth,
		AutosuggestEnabled: this.Config.ShellAutosuggestEnabled,
		AutosuggestChan:    make(chan *AutosuggestResult),
		Color:              colorScheme,
	}

	shellState.Prompt.SetTerminalWidth(termWidth)
	shellState.Prompt.SetColor(colorScheme.Prompt)

	// clear out any existing output to hide the PS1 export stuff
	clearByteChan(childOutReader, 100*time.Millisecond)
	fmt.Fprintf(childIn, "\n")

	// start
	shellState.Mux()
}

func rgbaToColorString(r, g, b, _ uint32) string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r/255, g/255, b/255)
}

// We expect the input string to end with a line containing "RUN: " followed by
// the command to run. If no command is found we return ""
func parseAquariumCommand(input string) string {
	if input == "" {
		return ""
	}
	lines := strings.Split(input, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "RUN: ") {
			return strings.TrimPrefix(line, "RUN: ")
		}
	}

	return ""
}

// TODO add a diagram of streams here
func (this *ShellState) Mux() {
	log.Printf("Started shell mux")
	parentInBuffer := []byte{}
	childOutBuffer := []byte{}

	for {
		select {
		case <-this.Butterfish.Ctx.Done():
			return

		// the terminal window resized and we got a SIGWINCH
		case <-this.Sigwinch:
			termWidth, _, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil {
				log.Printf("Error getting terminal size after SIGWINCH: %s", err)
			}
			log.Printf("Got SIGWINCH with new width %d", termWidth)
			this.TerminalWidth = termWidth
			this.Prompt.SetTerminalWidth(termWidth)
			if this.AutosuggestBuffer != nil {
				this.AutosuggestBuffer.SetTerminalWidth(termWidth)
			}
			if this.Command != nil {
				this.Command.SetTerminalWidth(termWidth)
			}

		// We received an autosuggest result from the autosuggest goroutine
		case result := <-this.AutosuggestChan:
			// request cursor position
			_, col := this.GetCursorPosition()
			var buffer *ShellBuffer

			// figure out which buffer we're autocompleting
			switch this.State {
			case statePrompting:
				buffer = this.Prompt
			case stateShell, stateNormal:
				buffer = this.Command
			default:
				log.Printf("Got autosuggest result in unexpected state %d", this.State)
				continue
			}

			this.ShowAutosuggest(buffer, result, col-1, this.TerminalWidth)

		// We finished with prompt output response, go back to normal mode
		case output := <-this.PromptOutputChan:
			this.History.Append(historyTypeLLMOutput, string(output.Data))

			// If there is child output waiting to be printed, print that now
			if len(childOutBuffer) > 0 {
				this.ParentOut.Write(childOutBuffer)
				this.History.Append(historyTypeShellOutput, string(childOutBuffer))
				childOutBuffer = []byte{}
			}

			// Get a new prompt
			this.ChildIn.Write([]byte("\n"))

			if this.AquariumMode {
				llmAsk := string(output.Data)
				if strings.Contains(llmAsk, "GOAL ACHIEVED") {
					log.Printf("Aquarium mode: goal achieved, exiting")
					this.AquariumMode = false
					this.setState(stateNormal)
					continue
				}
				if strings.Contains(llmAsk, "GOAL FAILED") {
					log.Printf("Aquarium mode: goal failed, exiting")
					this.AquariumMode = false
					this.setState(stateNormal)
					continue
				}

				aquariumCmd := parseAquariumCommand(llmAsk)
				if aquariumCmd != "" {
					// Execute the given aquarium command on the local shell
					log.Printf("Aquarium mode: running command: %s", aquariumCmd)
					this.AquariumBuffer = ""
					this.PromptSuffixCounter = 0
					this.setState(stateNormal)
					fmt.Fprintf(this.ChildIn, "%s\n", aquariumCmd)
					continue
				}

				this.PromptSuffixCounter = -10000
			}

			this.RequestAutosuggest(0, "")
			this.setState(stateNormal)

		case childOutMsg := <-this.ChildOutReader:
			if childOutMsg == nil {
				log.Println("Child out reader closed")
				this.Butterfish.Cancel()
				return
			}

			//log.Printf("Got child output:\n%s", prettyHex(childOutMsg.Data))

			lastStatus, prompts, childOutStr := ParsePS1(string(childOutMsg.Data))
			if prompts != 0 {
				log.Printf("Child exited with status %d", lastStatus)
			}
			this.PromptSuffixCounter += prompts

			// If we're actively printing a response we buffer child output
			if this.State == statePromptResponse {
				childOutBuffer = append(childOutBuffer, childOutMsg.Data...)
				continue
			}

			if this.AquariumMode {
				this.AquariumBuffer += childOutStr
			}

			// If we're getting child output while typing in a shell command, this
			// could mean the user is paging through old commands, or doing a tab
			// completion, or something unknown, so we don't want to add to history.
			if this.State != stateShell {
				this.History.Append(historyTypeShellOutput, childOutStr)
			}
			this.ParentOut.Write([]byte(childOutStr))

			if this.AquariumMode && this.PromptSuffixCounter >= 2 {
				// move cursor to the beginning of the line and clear the line
				fmt.Fprintf(this.ParentOut, "\r%s", ESC_CLEAR)
				this.AquariumCommandResponse(lastStatus, this.AquariumBuffer)
				this.AquariumBuffer = ""
				this.PromptSuffixCounter = 0
			}

		case parentInMsg := <-this.ParentInReader:
			if parentInMsg == nil {
				log.Println("Parent in reader closed")
				this.Butterfish.Cancel()
				return
			}

			data := parentInMsg.Data

			// include any cached data
			if len(parentInBuffer) > 0 {
				data = append(parentInBuffer, data...)
				parentInBuffer = []byte{}
			}

			// If we've started an ANSI escape sequence, it might not be complete
			// yet, so we need to cache it and wait for the next message
			if incompleteAnsiSequence(data) {
				parentInBuffer = append(parentInBuffer, data...)
				continue
			}

			for {
				leftover := this.InputFromParent(this.Butterfish.Ctx, data)

				if leftover == nil || len(leftover) == 0 {
					break
				}
				if len(leftover) == len(data) {
					// nothing was consumed, we buffer and try again later
					parentInBuffer = append(parentInBuffer, leftover...)
					break
				}

				// go again with the leftover data
				data = leftover
			}
		}
	}
}

func (this *ShellState) InputFromParent(ctx context.Context, data []byte) []byte {
	hasCarriageReturn := bytes.Contains(data, []byte{'\r'})

	switch this.State {
	case statePromptResponse:
		// Ctrl-C while receiving prompt
		if data[0] == 0x03 {
			this.PromptResponseCancel()
			this.PromptResponseCancel = nil
			return data[1:]
		}

		// If we're in the middle of a prompt response we ignore all other input
		return data

	case stateNormal:
		if HasRunningChildren() {
			// If we have running children then the shell is running something,
			// so just forward the input.
			this.ChildIn.Write(data)
			return nil
		}

		// Check if the first character is uppercase or a bang
		// TODO handle the case where this input is more than a single character, contains other stuff like carriage return, etc
		if unicode.IsUpper(rune(data[0])) || data[0] == '!' {
			this.setState(statePrompting)
			this.Prompt.Clear()
			this.Prompt.Write(string(data))

			// Write the actual prompt start
			color := this.Color.Prompt
			if data[0] == '!' {
				color = this.Color.PromptAction
			}
			this.Prompt.SetColor(color)
			fmt.Fprintf(this.ParentOut, "%s%s", color, data)

			// We're starting a prompt managed here in the wrapper, so we want to
			// get the cursor position
			_, col := this.GetCursorPosition()
			this.Prompt.SetPromptLength(col - 1 - this.Prompt.Size())
			return data[1:]

		} else if data[0] == '\t' { // user is asking to fill in an autosuggest
			if this.LastAutosuggest != "" {
				this.RealizeAutosuggest(this.Command, true, this.Color.Command)
				this.setState(stateShell)
				return data[1:]
			} else {
				// no last autosuggest found, just forward the tab
				this.ChildIn.Write(data)
			}
			return data[1:]

		} else if data[0] == '\r' {
			this.ChildIn.Write(data)
			return data[1:]

		} else {
			this.Command = NewShellBuffer()
			this.Command.Write(string(data))

			if this.Command.Size() > 0 {
				// this means that the command is not empty, i.e. the input wasn't
				// some control character
				this.RefreshAutosuggest(data, this.Command, this.Color.Command)
				this.setState(stateShell)
			} else {
				this.ClearAutosuggest(this.Color.Command)
			}

			this.ParentOut.Write([]byte(this.Color.Command))
			this.ChildIn.Write(data)
		}

	case statePrompting:
		// check if the input contains a newline
		if hasCarriageReturn {
			this.ClearAutosuggest(this.Color.Command)
			index := bytes.Index(data, []byte{'\r'})
			toAdd := data[:index]
			toPrint := this.Prompt.Write(string(toAdd))

			this.ParentOut.Write(toPrint)
			this.ParentOut.Write([]byte("\n\r"))

			promptStr := this.Prompt.String()
			if promptStr[0] == '!' {
				this.AquariumStart()
			} else if this.AquariumMode {
				this.AquariumChat()
			} else {
				this.SendPrompt()
			}
			return data[index+1:]

		} else if data[0] == '\t' { // user is asking to fill in an autosuggest
			// Tab was pressed, fill in lastAutosuggest
			if this.LastAutosuggest != "" {
				this.RealizeAutosuggest(this.Prompt, false, this.Color.Prompt)
			} else {
				// no last autosuggest found, just forward the tab
				this.ParentOut.Write(data)
			}

			return data[1:]

		} else if data[0] == 0x03 { // Ctrl-C
			toPrint := this.Prompt.Clear()
			this.ParentOut.Write(toPrint)
			this.ParentOut.Write([]byte(this.Color.Command))
			this.setState(stateNormal)

		} else { // otherwise user is typing a prompt
			toPrint := this.Prompt.Write(string(data))
			this.ParentOut.Write(toPrint)
			this.RefreshAutosuggest(data, this.Prompt, this.Color.Command)

			if this.Prompt.Size() == 0 {
				this.ParentOut.Write([]byte(this.Color.Command)) // reset color
				this.setState(stateNormal)
			}
		}

	case stateShell:
		if hasCarriageReturn { // user is submitting a command
			this.ClearAutosuggest(this.Color.Command)

			this.setState(stateNormal)

			index := bytes.Index(data, []byte{'\r'})
			this.ChildIn.Write(data[:index+1])
			this.History.Append(historyTypeShellInput, this.Command.String())
			this.Command = NewShellBuffer()

			return data[index+1:]

		} else if data[0] == '\t' { // user is asking to fill in an autosuggest
			// Tab was pressed, fill in lastAutosuggest
			if this.LastAutosuggest != "" {
				this.RealizeAutosuggest(this.Command, true, this.Color.Command)
			} else {
				// no last autosuggest found, just forward the tab
				this.ChildIn.Write(data)
			}
			return data[1:]

		} else { // otherwise user is typing a command
			this.Command.Write(string(data))
			this.RefreshAutosuggest(data, this.Command, this.Color.Command)
			this.ChildIn.Write(data)
			if this.Command.Size() == 0 {
				this.setState(stateNormal)
			}
		}

	default:
		panic("Unknown state")
	}

	return nil
}

// We want to queue up the prompt response, which does the processing (except
// for actually printing it). The processing like adding to history or
// executing the next step in aquarium mode. We have to do this in a goroutine
// because otherwise we would block the main thread.
func (this *ShellState) SendPromptResponse(data string) {
	go func() {
		this.PromptOutputChan <- &byteMsg{Data: []byte(data)}
	}()
}

func (this *ShellState) PrintStatus() {
	text := fmt.Sprintf("You're using Butterfish Shell Mode\n%s\n\n", this.Butterfish.Config.BuildInfo)

	text += fmt.Sprintf("Prompting model:       %s\n", this.Butterfish.Config.ShellPromptModel)
	text += fmt.Sprintf("Prompt history window: %d bytes\n", this.Butterfish.Config.ShellPromptHistoryWindow)
	text += fmt.Sprintf("Command prompt:        %s\n", this.Butterfish.Config.ShellCommandPrompt)
	text += fmt.Sprintf("Autosuggest:           %t\n", this.Butterfish.Config.ShellAutosuggestEnabled)
	text += fmt.Sprintf("Autosuggest model:     %s\n", this.Butterfish.Config.ShellAutosuggestModel)
	text += fmt.Sprintf("Autosuggest timeout:   %s\n", this.Butterfish.Config.ShellAutosuggestTimeout)
	text += fmt.Sprintf("Autosuggest history:   %d bytes\n", this.Butterfish.Config.ShellAutosuggestHistoryWindow)
	fmt.Fprintf(this.PromptAnswerWriter, "%s%s%s", this.Color.Answer, text, this.Color.Command)
	this.SendPromptResponse(text)
}

func (this *ShellState) PrintHelp() {
	text := `You're using the Butterfish Shell Mode, which means you have a Butterfish wrapper around your normal shell. Here's how you use it:

	- Type a normal command, like "ls -l" and press enter to execute it
	- Start a command with a capital letter to send it to GPT, like "How do I find local .py files?"
	- Autosuggest will print command completions, press tab to fill them in
	- GPT will be able to see your shell history, so you can ask contextual questions like "why didn't my last command work?"
	- Type "Status" to show the current Butterfish configuration
	- Type "History" to show the recent history that will be sent to GPT
`
	fmt.Fprintf(this.PromptAnswerWriter, "%s%s%s", this.Color.Answer, text, this.Color.Command)
	this.SendPromptResponse(text)
}

func (this *ShellState) PrintHistory() {
	historyBlocks := this.History.GetLastNBytes(this.Butterfish.Config.ShellPromptHistoryWindow, 2048)
	strBuilder := strings.Builder{}

	for _, block := range historyBlocks {
		// block header
		strBuilder.WriteString(fmt.Sprintf("%s%s\n", this.Color.Aquarium, HistoryTypeToString(block.Type)))
		blockColor := this.Color.Command
		switch block.Type {
		case historyTypePrompt:
			blockColor = this.Color.Prompt
		case historyTypeLLMOutput:
			blockColor = this.Color.Answer
		case historyTypeShellInput:
			blockColor = this.Color.PromptAction
		}

		strBuilder.WriteString(fmt.Sprintf("%s%s\n", blockColor, block.Content))
	}

	this.History.LogRecentHistory()
	fmt.Fprintf(this.PromptAnswerWriter, "%s%s", strBuilder.String(), this.Color.Command)
	this.SendPromptResponse("")
}

const aquariumSystemMessage = "You are an agent attempting to achieve a goal in Aquarium mode. In Aquarium mode, I will give you a goal, and you will give me unix commands to execute. If a command is given, it should be on the final line and preceded with 'RUN: '. I will give you the results of the command. If we haven't reached our goal, you will then continue to give me commands to execute to reach that goal. If there is significant ambiguity then you can ask me questions. You must verify that the goal is achieved. When finished, respond with exactly 'GOAL ACHIEVED' or 'GOAL FAILED' if it isn't possible. If you don't have a goal respond with 'GOAL ACHIEVED'."

func (this *ShellState) AquariumStart() {
	this.AquariumMode = true

	// Get the prompt after the bang
	prompt := this.Prompt.String()[1:]
	prompt = fmt.Sprintf("This is your goal: %s", prompt)
	log.Printf("Starting Aquarium mode: %s", prompt)
	this.Prompt.Clear()

	historyBlocks := this.History.GetLastNBytes(this.Butterfish.Config.ShellPromptHistoryWindow, 2048)
	requestCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	this.PromptResponseCancel = cancel

	request := &util.CompletionRequest{
		Ctx:           requestCtx,
		Prompt:        prompt,
		Model:         this.Butterfish.Config.ShellPromptModel,
		MaxTokens:     2048,
		Temperature:   0.7,
		HistoryBlocks: historyBlocks,
		SystemMessage: aquariumSystemMessage,
	}

	this.History.Append(historyTypePrompt, prompt)
	log.Printf("Aquarium prompt: %s\n", prompt)

	// we run this in a goroutine so that we can still receive input
	// like Ctrl-C while waiting for the response
	go CompletionRoutine(request, this.Butterfish.LLMClient,
		this.PromptAnswerWriter, this.PromptOutputChan,
		this.Color.Aquarium, this.Color.Error)
}

func (this *ShellState) AquariumChat() {
	prompt := this.Prompt.String()
	this.Prompt.Clear()

	log.Printf("Aquarium chat: %s\n", prompt)
	historyBlocks := this.History.GetLastNBytes(this.Butterfish.Config.ShellPromptHistoryWindow, 2048)
	requestCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	this.PromptResponseCancel = cancel

	request := &util.CompletionRequest{
		Ctx:           requestCtx,
		Prompt:        prompt,
		Model:         this.Butterfish.Config.ShellPromptModel,
		MaxTokens:     2048,
		Temperature:   0.7,
		HistoryBlocks: historyBlocks,
		SystemMessage: aquariumSystemMessage,
	}

	// we run this in a goroutine so that we can still receive input
	// like Ctrl-C while waiting for the response
	go CompletionRoutine(request, this.Butterfish.LLMClient,
		this.PromptAnswerWriter, this.PromptOutputChan,
		this.Color.Aquarium, this.Color.Error)
}

func (this *ShellState) AquariumCommandResponse(status int, output string) {
	log.Printf("Aquarium response: %d\n", status)
	historyBlocks := this.History.GetLastNBytes(this.Butterfish.Config.ShellPromptHistoryWindow, 2048)
	requestCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	this.PromptResponseCancel = cancel

	prompt := fmt.Sprintf("%s\nExit code: %d\n", output, status)

	request := &util.CompletionRequest{
		Ctx:           requestCtx,
		Prompt:        prompt,
		Model:         this.Butterfish.Config.ShellPromptModel,
		MaxTokens:     2048,
		Temperature:   0.7,
		HistoryBlocks: historyBlocks,
		SystemMessage: aquariumSystemMessage,
	}

	// we run this in a goroutine so that we can still receive input
	// like Ctrl-C while waiting for the response
	go CompletionRoutine(request, this.Butterfish.LLMClient,
		this.PromptAnswerWriter, this.PromptOutputChan,
		this.Color.Aquarium, this.Color.Error)
}

func (this *ShellState) SendPrompt() {
	this.setState(statePromptResponse)

	promptStr := strings.ToLower(this.Prompt.String())
	promptStr = strings.TrimSpace(promptStr)

	switch promptStr {
	case "status":
		this.PrintStatus()
		return
	case "help":
		this.PrintHelp()
		return
	case "history":
		this.PrintHistory()
		return
	}

	historyBlocks := this.History.GetLastNBytes(this.Butterfish.Config.ShellPromptHistoryWindow, 512)
	requestCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	this.PromptResponseCancel = cancel

	sysMsg, err := this.Butterfish.PromptLibrary.GetPrompt(prompt.PromptShellSystemMessage)
	if err != nil {
		log.Printf("Error getting system message prompt: %s", err)
		this.setState(stateNormal)
		return
	}

	request := &util.CompletionRequest{
		Ctx:           requestCtx,
		Prompt:        this.Prompt.String(),
		Model:         this.Butterfish.Config.ShellPromptModel,
		MaxTokens:     512,
		Temperature:   0.7,
		HistoryBlocks: historyBlocks,
		SystemMessage: sysMsg,
	}

	this.History.Append(historyTypePrompt, this.Prompt.String())

	// we run this in a goroutine so that we can still receive input
	// like Ctrl-C while waiting for the response
	go CompletionRoutine(request, this.Butterfish.LLMClient,
		this.PromptAnswerWriter, this.PromptOutputChan, this.Color.Answer,
		this.Color.Error)

	this.Prompt.Clear()
}

func CompletionRoutine(request *util.CompletionRequest, client LLM, writer io.Writer, outputChan chan *byteMsg, normalColor, errorColor string) {
	fmt.Fprintf(writer, "%s", normalColor)
	output, err := client.CompletionStream(request, writer)

	toSend := []byte{}
	if output != "" {
		toSend = []byte(output)
	}

	if err != nil {
		errStr := fmt.Sprintf("Error prompting LLM: %s\n", err)

		// This error means the user needs to set up a subscription, give advice
		if strings.Contains(errStr, ERR_429) {
			errStr = fmt.Sprintf("%s\n%s", errStr, ERR_429_HELP)
		}

		log.Printf("%s", errStr)

		if !strings.Contains(errStr, "context canceled") {
			fmt.Fprintf(writer, "%s%s", errorColor, errStr)
			// We want to put the error message in the history as well
			toSend = append(toSend, []byte(errStr)...)
		}
	}

	if len(toSend) > 0 {
		// send any output + error for processing (e.g. adding to history)
		outputChan <- &byteMsg{Data: toSend}
	}
}

// When the user presses tab or a similar hotkey, we want to turn the
// autosuggest into a real command
func (this *ShellState) RealizeAutosuggest(buffer *ShellBuffer, sendToChild bool, colorStr string) {
	log.Printf("Realizing autosuggest: %s", this.LastAutosuggest)

	writer := this.ParentOut
	if sendToChild {
		writer = this.ChildIn
	}

	// If we're not at the end of the line, we write out the remaining command
	// before writing the autosuggest
	jumpforward := buffer.Size() - buffer.Cursor()
	if jumpforward > 0 {
		// go right for the length of the suffix
		for i := 0; i < jumpforward; i++ {
			// move cursor right
			fmt.Fprintf(writer, "\x1b[C")
			buffer.Write("\x1b[C")
		}
	}

	// set color
	if colorStr != "" {
		this.ParentOut.Write([]byte(colorStr))
	}

	// Write the autosuggest
	fmt.Fprintf(writer, "%s", this.LastAutosuggest)
	buffer.Write(this.LastAutosuggest)

	// clear the autosuggest now that we've used it
	this.LastAutosuggest = ""
}

// We have a pending autosuggest and we've just received the cursor location
// from the terminal. We can now render the autosuggest (in the greyed out
// style)
func (this *ShellState) ShowAutosuggest(
	buffer *ShellBuffer, result *AutosuggestResult, cursorCol int, termWidth int) {

	if result.Suggestion == "" {
		// no suggestion
		return
	}

	//log.Printf("ShowAutosuggest: %s", result.Suggestion)

	if result.Command != buffer.String() {
		// this is an old result, it doesn't match the current command buffer
		log.Printf("Autosuggest result is old, ignoring. Expected: %s, got: %s", buffer.String(), result.Command)
		return
	}

	if strings.Contains(result.Suggestion, "\n") {
		// if result.Suggestion has newlines then discard it
		return
	}

	if result.Suggestion == this.LastAutosuggest {
		// if the suggestion is the same as the last one, ignore it
		return
	}

	if result.Command != "" &&
		!strings.HasPrefix(
			strings.ToLower(result.Suggestion),
			strings.ToLower(result.Command)) {
		// test that the command is equal to the beginning of the suggestion
		log.Printf("Autosuggest result is invalid, ignoring")
		return
	}

	if result.Suggestion == buffer.String() {
		// if the suggestion is the same as the command, ignore it
		return
	}

	// Print out autocomplete suggestion
	cmdLen := buffer.Size()
	suggToAdd := result.Suggestion[cmdLen:]
	jumpForward := cmdLen - buffer.Cursor()

	this.LastAutosuggest = suggToAdd

	this.AutosuggestBuffer = NewShellBuffer()
	this.AutosuggestBuffer.SetPromptLength(cursorCol)
	this.AutosuggestBuffer.SetTerminalWidth(termWidth)

	// Use autosuggest buffer to get the bytes to write the greyed out
	// autosuggestion and then move the cursor back to the original position
	buf := this.AutosuggestBuffer.WriteAutosuggest(suggToAdd, jumpForward, this.Color.Autosuggest)

	this.ParentOut.Write([]byte(buf))
}

// Update autosuggest when we receive new data
func (this *ShellState) RefreshAutosuggest(newData []byte, buffer *ShellBuffer, colorStr string) {
	// if we're typing out the exact autosuggest, and we haven't moved the cursor
	// backwards in the buffer, then we can just append and adjust the
	// autosuggest
	if buffer.Size() > 0 &&
		buffer.Size() == buffer.Cursor() &&
		bytes.HasPrefix([]byte(this.LastAutosuggest), newData) {
		this.LastAutosuggest = this.LastAutosuggest[len(newData):]
		if colorStr != "" {
			this.ParentOut.Write([]byte(colorStr))
		}
		this.AutosuggestBuffer.EatAutosuggestRune()
		return
	}

	// otherwise, clear the autosuggest
	this.ClearAutosuggest(colorStr)

	// and request a new one
	if this.State == stateShell || this.State == statePrompting {
		this.RequestAutosuggest(
			this.Butterfish.Config.ShellAutosuggestTimeout, buffer.String())
	}
}

func (this *ShellState) ClearAutosuggest(colorStr string) {
	if this.LastAutosuggest == "" {
		// there wasn't actually a last autosuggest, so nothing to clear
		return
	}

	this.LastAutosuggest = ""
	this.ParentOut.Write(this.AutosuggestBuffer.ClearLast(colorStr))
	this.AutosuggestBuffer = nil
}

func (this *ShellState) RequestAutosuggest(delay time.Duration, command string) {
	if !this.AutosuggestEnabled {
		return
	}

	if this.AutosuggestCancel != nil {
		// clear out a previous request
		this.AutosuggestCancel()
	}
	this.AutosuggestCtx, this.AutosuggestCancel = context.WithCancel(context.Background())

	// if command is only whitespace, don't bother sending it
	if len(command) > 0 && strings.TrimSpace(command) == "" {
		return
	}

	historyBlocks := HistoryBlocksToString(this.History.GetLastNBytes(this.Butterfish.Config.ShellAutosuggestHistoryWindow, 2048))

	var llmPrompt string
	var err error

	if len(command) == 0 {
		// command completion when we haven't started a command
		llmPrompt, err = this.Butterfish.PromptLibrary.GetPrompt(prompt.PromptShellAutosuggestNewCommand,
			"history", historyBlocks)
	} else if !unicode.IsUpper(rune(command[0])) {
		// command completion when we have started typing a command
		llmPrompt, err = this.Butterfish.PromptLibrary.GetPrompt(prompt.PromptShellAutosuggestCommand,
			"history", historyBlocks,
			"command", command)
	} else {
		// prompt completion, like we're asking a question
		llmPrompt, err = this.Butterfish.PromptLibrary.GetPrompt(prompt.PromptShellAutosuggestPrompt,
			"history", historyBlocks,
			"command", command)
	}

	if err != nil {
		log.Printf("Error getting prompt from library: %s", err)
		return
	}

	go RequestCancelableAutosuggest(
		this.AutosuggestCtx, delay,
		command, llmPrompt,
		this.Butterfish.LLMClient,
		this.Butterfish.Config.ShellAutosuggestModel,
		this.AutosuggestChan)
}

func RequestCancelableAutosuggest(
	ctx context.Context,
	delay time.Duration,
	currCommand string,
	prompt string,
	llmClient LLM,
	model string,
	autosuggestChan chan<- *AutosuggestResult) {

	if delay > 0 {
		time.Sleep(delay)
	}
	if ctx.Err() != nil {
		return
	}

	request := &util.CompletionRequest{
		Ctx:         ctx,
		Prompt:      prompt,
		Model:       model,
		MaxTokens:   256,
		Temperature: 0.7,
	}

	output, err := llmClient.Completion(request)
	if err != nil && !strings.Contains(err.Error(), "context canceled") {
		log.Printf("Autosuggest error: %s", err)
		if strings.Contains(err.Error(), ERR_429) {
			log.Printf(ERR_429_HELP)
		}
		return
	}

	// Clean up wrapping whitespace
	output = strings.TrimSpace(output)

	// if output is wrapped in quotes, remove quotes
	if len(output) > 1 && output[0] == '"' && output[len(output)-1] == '"' {
		output = output[1 : len(output)-1]
	}

	// Clean up wrapping whitespace
	output = strings.TrimSpace(output)

	autoSuggest := &AutosuggestResult{
		Command:    currCommand,
		Suggestion: output,
	}
	autosuggestChan <- autoSuggest
}

// Given a PID, this function identifies all the child PIDs of the given PID
// and returns them as a slice of ints.
func countChildPids(pid int) (int, error) {
	// Get all the processes
	processes, err := ps.Processes()
	if err != nil {
		return -1, err
	}

	// Keep a set of pids, loop through and add children to the set, keep
	// looping until the set stops growing.
	pids := make(map[int]bool)
	pids[pid] = true
	for {
		// Keep track of how many pids we've added in this iteration
		added := 0

		// Loop through all the processes
		for _, p := range processes {
			// If the process is a child of one of the pids we're tracking,
			// add it to the set.
			if pids[p.PPid()] && !pids[p.Pid()] {
				pids[p.Pid()] = true
				added++
			}
		}

		// If we didn't add any new pids, we're done.
		if added == 0 {
			break
		}
	}

	// subtract 1 because we don't want to count the parent pid
	return len(pids) - 1, nil
}

func HasRunningChildren() bool {
	// get this process's pid
	pid := os.Getpid()

	// get the number of child processes
	count, err := countChildPids(pid)
	if err != nil {
		log.Printf("Error counting child processes: %s", err)
		return false
	}

	// we expect 1 child because the shell is running
	if count > 1 {
		return true
	}
	return false
}
