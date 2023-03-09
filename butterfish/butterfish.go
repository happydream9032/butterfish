package butterfish

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/creack/pty"
	"github.com/mitchellh/go-homedir"
	"golang.org/x/term"

	"github.com/bakks/butterfish/bubbles/console"
	"github.com/bakks/butterfish/embedding"
	"github.com/bakks/butterfish/prompt"
	"github.com/bakks/butterfish/util"
)

// Main driver for the Butterfish set of command line tools. These are tools
// for using AI capabilities on the command line.

type ButterfishConfig struct {
	Verbose           bool
	OpenAIToken       string
	LLMClient         LLM
	ColorScheme       *ColorScheme
	Styles            *styles
	PromptLibraryPath string
	PromptLibrary     PromptLibrary

	GencmdModel          string
	GencmdTemperature    float32
	GencmdMaxTokens      int
	ExeccheckModel       string
	ExeccheckTemperature float32
	ExeccheckMaxTokens   int
	SummarizeModel       string
	SummarizeTemperature float32
	SummarizeMaxTokens   int
}

type PromptLibrary interface {
	GetPrompt(name string, args ...string) (string, error)
}

type LLM interface {
	CompletionStream(request *util.CompletionRequest, writer io.Writer) (string, error)
	Completion(request *util.CompletionRequest) (string, error)
	Embeddings(ctx context.Context, input []string) ([][]float64, error)
	Edits(ctx context.Context, content, instruction, model string, temperature float32) (string, error)
}

type ButterfishCtx struct {
	Ctx    context.Context    // global context, should be passed through to other calls
	Cancel context.CancelFunc // cancel function for the global context
	Out    io.Writer          // output writer

	Config          *ButterfishConfig            // configuration
	InConsoleMode   bool                         // true if we're running in console mode
	PromptLibrary   PromptLibrary                // library of prompts
	LLMClient       LLM                          // GPT client
	CommandRegister string                       // landing space for generated commands
	VectorIndex     embedding.FileEmbeddingIndex // embedding index for searching local files

	ConsoleCmdChan   <-chan string    // channel for console commands
	ClientController ClientController // client controller
}

type ColorScheme struct {
	Foreground string // neutral foreground color
	Background string
	Error      string // should be reddish
	Color1     string // should be greenish
	Color2     string
	Color3     string
	Color4     string
	Color5     string
	Color6     string
	Grey       string
}

// Gruvbox Colorscheme
// from https://github.com/morhetz/gruvbox
var GruvboxDark = ColorScheme{
	Foreground: "#ebdbb2",
	Background: "#282828",
	Error:      "#fb4934", // red
	Color1:     "#b8bb26", // green
	Color2:     "#fabd2f", // yellow
	Color3:     "#83a598", // blue
	Color4:     "#d3869b", // magenta
	Color5:     "#8ec07c", // cyan
	Color6:     "#fe8019", // orange
	Grey:       "#928374", // gray
}

var GruvboxLight = ColorScheme{
	Foreground: "#7C6F64",
	Background: "#FBF1C7",
	Error:      "#CC241D",
	Color1:     "#98971A",
	Color2:     "#D79921",
	Color3:     "#458588",
	Color4:     "#B16286",
	Color5:     "#689D6A",
	Color6:     "#D65D0E",
	Grey:       "#928374",
}

const BestCompletionModel = "gpt-3.5-turbo"
const BestAutosuggestModel = "text-davinci-003"

func MakeButterfishConfig() *ButterfishConfig {
	colorScheme := &GruvboxDark

	return &ButterfishConfig{
		Verbose:              false,
		ColorScheme:          colorScheme,
		Styles:               ColorSchemeToStyles(colorScheme),
		GencmdModel:          BestCompletionModel,
		GencmdTemperature:    0.6,
		GencmdMaxTokens:      512,
		ExeccheckModel:       BestCompletionModel,
		ExeccheckTemperature: 0.6,
		ExeccheckMaxTokens:   512,
		SummarizeModel:       BestCompletionModel,
		SummarizeTemperature: 0.7,
		SummarizeMaxTokens:   1024,
	}
}

// Data type for passing byte chunks from a wrapped command around
type byteMsg struct {
	Data []byte
}

func NewByteMsg(data []byte) *byteMsg {
	buf := make([]byte, len(data))
	copy(buf, data)
	return &byteMsg{
		Data: buf,
	}
}

// Given an io.Reader we write byte chunks to a channel
func readerToChannel(input io.Reader, c chan<- *byteMsg) {
	buf := make([]byte, 1024*16)

	// Loop indefinitely
	for {
		// Read from stream
		n, err := input.Read(buf)

		// Check for error
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from file: %s\n", err)
			}
			break
		}

		c <- NewByteMsg(buf[:n])
	}

	// Close the channel
	close(c)
}

// from https://github.com/acarl005/stripansi/blob/master/stripansi.go
const ansiPattern = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"

var ansiRegexp = regexp.MustCompile(ansiPattern)

// Strip ANSI tty control codes out of a string
func stripANSI(str string) string {
	return ansiRegexp.ReplaceAllString(str, "")
}

// Function for filtering out non-printable characters from a string
func filterNonPrintable(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\t': // we don't want to filter these out though
			return r

		default:
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}
	}, s)
}

func sanitizeTTYData(data []byte) []byte {
	return []byte(filterNonPrintable(stripANSI(string(data))))
}

func ptyCommand(ctx context.Context, command []string) (*os.File, func() error, error) {
	// Create arbitrary command.
	var cmd *exec.Cmd
	if len(command) > 1 {
		cmd = exec.CommandContext(ctx, command[0], command[1:]...)
	} else {
		cmd = exec.CommandContext(ctx, command[0])
	}

	// Start the command with a pty.
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, err
	}

	// Handle pty size.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize.

	// Set stdin in raw mode.
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		ptmx.Close()
		signal.Stop(ch)
		close(ch)
		return nil, nil, err
	}

	cleanup := func() error {
		err := ptmx.Close()
		if err != nil {
			return err
		}

		signal.Stop(ch)
		close(ch)

		return term.Restore(int(os.Stdin.Fd()), oldState)
	}

	return ptmx, cleanup, nil
}

// Based on example at https://github.com/creack/pty
// Apparently you can't start a shell like zsh without
// this more complex command execution
func wrapCommand(ctx context.Context, cancel context.CancelFunc, command []string, client *IPCClient) error {
	ptmx, cleanup, err := ptyCommand(ctx, command)
	if err != nil {
		return err
	}
	defer cleanup()

	parentIn := make(chan *byteMsg)
	childOut := make(chan *byteMsg)
	remoteIn := make(chan *byteMsg)

	// Read from this process Stdin and write to stdinChannel
	go readerToChannel(os.Stdin, parentIn)
	// Read from pty Stdout and write to stdoutChannel
	go readerToChannel(ptmx, childOut)
	// Read from remote
	go packageRPCStream(client, remoteIn)

	client.SendWrappedCommand(strings.Join(command, " "))

	wrappingMultiplexer(ctx, cancel, client, ptmx, parentIn, remoteIn, childOut)

	return nil
}

func (this *ButterfishCtx) CalculateEmbeddings(ctx context.Context, content []string) ([][]float64, error) {
	return this.LLMClient.Embeddings(ctx, content)
}

// A local printf that writes to the butterfishctx out using a lipgloss style
func (this *ButterfishCtx) StylePrintf(style lipgloss.Style, format string, a ...any) {
	str := util.MultilineLipglossRender(style, fmt.Sprintf(format, a...))
	this.Out.Write([]byte(str))
}

func (this *ButterfishCtx) StyleSprintf(style lipgloss.Style, format string, a ...any) string {
	return util.MultilineLipglossRender(style, fmt.Sprintf(format, a...))
}

func (this *ButterfishCtx) Printf(format string, a ...any) {
	this.StylePrintf(this.Config.Styles.Foreground, format, a...)
}

func (this *ButterfishCtx) ErrorPrintf(format string, a ...any) {
	this.StylePrintf(this.Config.Styles.Error, format, a...)
}

// Ensure we have a vector index object, idempotent
func (this *ButterfishCtx) initVectorIndex(pathsToLoad []string) error {
	if this.VectorIndex != nil {
		return nil
	}

	out := util.NewStyledWriter(this.Out, this.Config.Styles.Foreground)
	index := embedding.NewDiskCachedEmbeddingIndex(this, out)

	if this.Config.Verbose {
		index.SetOutput(this.Out)
	}

	this.VectorIndex = index

	if !this.InConsoleMode {
		// if we're running from the command line then we first load the curr
		// dir index
		if pathsToLoad == nil || len(pathsToLoad) == 0 {
			pathsToLoad = []string{"."}
		}

		err := this.VectorIndex.LoadPaths(this.Ctx, pathsToLoad)
		if err != nil {
			return err
		}
	}

	return nil
}

func (this *ButterfishCtx) printError(err error, prefix ...string) {
	if len(prefix) > 0 {
		fmt.Fprintf(this.Out, "%s error: %s\n", prefix[0], err.Error())
	} else {
		fmt.Fprintf(this.Out, "Error: %s\n", err.Error())
	}
}

type styles struct {
	Question   lipgloss.Style
	Answer     lipgloss.Style
	Go         lipgloss.Style
	Summarize  lipgloss.Style
	Highlight  lipgloss.Style
	Prompt     lipgloss.Style
	Error      lipgloss.Style
	Foreground lipgloss.Style
	Grey       lipgloss.Style
}

func (this *styles) PrintTestColors() {
	fmt.Println(this.Question.Render("Question"))
	fmt.Println(this.Answer.Render("Answer"))
	fmt.Println(this.Go.Render("Go"))
	fmt.Println(this.Summarize.Render("Summarize"))
	fmt.Println(this.Highlight.Render("Highlight"))
	fmt.Println(this.Prompt.Render("Prompt"))
	fmt.Println(this.Error.Render("Error"))
	fmt.Println(this.Foreground.Render("Foreground"))
	fmt.Println(this.Grey.Render("Grey"))
}

func ColorSchemeToStyles(colorScheme *ColorScheme) *styles {
	return &styles{
		Question:   lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Color4)),
		Answer:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Color2)),
		Go:         lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Color5)),
		Highlight:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Color2)),
		Summarize:  lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Color2)),
		Prompt:     lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Color4)),
		Error:      lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Error)),
		Foreground: lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Foreground)),
		Grey:       lipgloss.NewStyle().Foreground(lipgloss.Color(colorScheme.Grey)),
	}
}

// Let's initialize our prompts. If we have a prompt library file, we'll load it.
// Either way, we'll then add the default prompts to the library, replacing
// loaded prompts only if OkToReplace is set on them. Then we save the library
// at the same path.
func NewDiskPromptLibrary(path string, verbose bool, writer io.Writer) (*prompt.DiskPromptLibrary, error) {
	promptLibrary := prompt.NewPromptLibrary(path, verbose, writer)
	loaded := false

	if promptLibrary.LibraryFileExists() {
		err := promptLibrary.Load()
		if err != nil {
			return nil, err
		}
		loaded = true
	}
	promptLibrary.ReplacePrompts(prompt.DefaultPrompts)
	promptLibrary.Save()

	if !loaded {
		fmt.Fprintf(writer, "Wrote prompt library at %s\n", path)
	}

	return promptLibrary, nil
}

func RunShell(ctx context.Context, config *ButterfishConfig, shell string) error {
	ptmx, ptyCleanup, err := ptyCommand(ctx, []string{shell})
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

// This holds a buffer that represents a tty shell buffer. Incoming data
// manipulates the buffer, for example the left arrow will move the cursor left,
// a backspace would erase the end of the buffer. Special characters that do not
// impact the cursor or printable characters will be ignored, for example 0x00
// is ignored.
type ShellBuffer struct {
	// The buffer itself
	buffer []rune
	cursor int
}

func (this *ShellBuffer) WriteRune(r rune) {
	// otherwise just add the character to the buffer
	if this.cursor == len(this.buffer) {
		this.buffer = append(this.buffer, r)
	} else {
		this.buffer = append(this.buffer[:this.cursor], append([]rune{r}, this.buffer[this.cursor:]...)...)
	}
	this.cursor++
}

func (this *ShellBuffer) Write(data string) {
	if len(data) == 0 {
		return
	}

	data = stripANSI(data)
	if len(data) == 0 {
		return
	}

	if len(data) >= 3 && data[0] == 0x1b && data[1] == 0x5b {
		if data[2] == 0x44 {
			// left arrow
			if this.cursor > 0 {
				this.cursor--
			}
			this.Write(data[3:])
			return
		}
		if data[2] == 0x43 {
			// right arrow
			if this.cursor < len(this.buffer) {
				this.cursor++
			}
			this.Write(data[3:])
			return
		}
	}

	for _, r := range data {
		switch int(r) {
		case 0x0a, 0x0d: // newline, carriage return
			this.WriteRune(r)

		case 0x08, 0x7f: // backspace
			if this.cursor > 0 && len(this.buffer) > 0 {
				this.buffer = append(this.buffer[:this.cursor-1], this.buffer[this.cursor:]...)
				this.cursor--
			}

		default:
			this.WriteRune(r)

		}
	}
}

func (this *ShellBuffer) String() string {
	return string(this.buffer)
}

func NewShellBuffer() *ShellBuffer {
	return &ShellBuffer{
		buffer: make([]rune, 0),
		cursor: 0,
	}
}

const (
	historyTypePrompt = iota
	historyTypeShellInput
	historyTypeShellOutput
	historyTypeLLMOutput
)

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

func (this *ShellHistory) Add(historyType int, block string) {
	buffer := NewShellBuffer()
	buffer.Write(block)
	this.Blocks = append(this.Blocks, HistoryBuffer{
		Type:    historyType,
		Content: buffer,
	})
}

func (this *ShellHistory) Append(historyType int, data string) {
	if len(this.Blocks) > 0 && this.Blocks[len(this.Blocks)-1].Type == historyType {
		this.Blocks[len(this.Blocks)-1].Content.Write(data)
	} else {
		this.Add(historyType, data)
	}
}

func (this *ShellHistory) NewBlock() {
	if len(this.Blocks) > 0 {
		this.Add(this.Blocks[len(this.Blocks)-1].Type, "")
	}
}

// Go back in history for a certain number of bytes.
// This truncates each block content to a maximum of 512 bytes.
func (this *ShellHistory) GetLastNBytes(numBytes int) []util.HistoryBlock {
	var blocks []util.HistoryBlock
	const truncateLength = 512

	for i := len(this.Blocks) - 1; i >= 0 && numBytes > 0; i-- {
		block := this.Blocks[i]
		content := block.Content.String()
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
	blocks := this.GetLastNBytes(2000)
	log.Printf("Recent history: =======================================")
	for _, block := range blocks {
		if block.Type == historyTypePrompt {
			log.Printf("Prompt: %s", block.Content)
		} else if block.Type == historyTypeShellInput {
			log.Printf("Shell input: %s", block.Content)
		} else if block.Type == historyTypeShellOutput {
			log.Printf("Shell output: %s", block.Content)
		} else if block.Type == historyTypeLLMOutput {
			log.Printf("LLM output: %s", block.Content)
		}
	}
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
)

type AutosuggestResult struct {
	Command    string
	Suggestion string
}

type ShellState struct {
	Butterfish *ButterfishCtx
	ParentOut  io.Writer
	ChildIn    io.Writer

	// The current state of the shell
	State              int
	ChildOutReader     chan *byteMsg
	ParentInReader     chan *byteMsg
	AutosuggestChan    chan *AutosuggestResult
	History            *ShellHistory
	PromptAnswerWriter io.Writer
	Prompt             string
	PromptStyle        lipgloss.Style
	Command            *ShellBuffer

	LastAutosuggest   string
	AutosuggestCtx    context.Context
	AutosuggestCancel context.CancelFunc
	AutosuggestStyle  lipgloss.Style
}

// TODO add a diagram of streams here
// States:
// 1. Normal
// 2. Prompting
// 3. Shell
func (this *ShellState) Mux() {
	for {
		select {
		case <-this.Butterfish.Ctx.Done():
			return

		case result := <-this.AutosuggestChan:
			if result.Command != this.Command.String() || result.Suggestion == "" {
				// this is an old result (or no suggestion appeared), ignore it
				continue
			}

			// if result.Suggestion has newlines then discard it
			if strings.Contains(result.Suggestion, "\n") {
				continue
			}

			// if the suggestion is the same as the last one, ignore it
			if result.Suggestion == this.LastAutosuggest {
				continue
			}

			this.LastAutosuggest = result.Suggestion

			log.Printf("Autosuggest result: %s", result.Suggestion)

			// test that the command is equal to the beginning of the suggestion
			if result.Command != "" && !strings.HasPrefix(result.Suggestion, result.Command) {
				continue
			}

			// Print out autocomplete suggestion
			cmdLen := len(this.Command.String())
			suggToAdd := result.Suggestion[cmdLen:]
			this.LastAutosuggest = suggToAdd
			rendered := this.AutosuggestStyle.Render(suggToAdd)
			this.ParentOut.Write([]byte(rendered))
			for i := 0; i < len(result.Suggestion)-cmdLen; i++ {
				this.ParentOut.Write(cursorLeft)
			}

		case childOutMsg := <-this.ChildOutReader:
			if childOutMsg == nil {
				log.Println("Child out reader closed")
				this.Butterfish.Cancel()
				return
			}
			//cleanData := sanitizeTTYData(childOutMsg.Data)
			//this.History.IdempotentAdd(historyTypeShellOutput, string(cleanData))
			this.History.Append(historyTypeShellOutput, string(childOutMsg.Data))
			//log.Printf("Child out: %s  0x%x", string(childOutMsg.Data), childOutMsg.Data)
			//this.History.LogRecentHistory()
			this.ParentOut.Write(childOutMsg.Data)

		case parentInMsg := <-this.ParentInReader:
			if parentInMsg == nil {
				log.Println("Parent in reader closed")
				this.Butterfish.Cancel()
				return
			}
			data := parentInMsg.Data
			this.InputFromParent(this.Butterfish.Ctx, data)
		}
	}
}

func (this *ShellState) InputFromParent(ctx context.Context, data []byte) {
	hasCarriageReturn := bytes.Contains(data, []byte{'\r'})

	switch this.State {
	case stateNormal:
		// check if the first character is uppercase
		// TODO handle the case where this input is more than a single character, contains other stuff like carriage return, etc
		if unicode.IsUpper(rune(data[0])) {
			this.State = statePrompting
			log.Printf("State change: normal -> prompting")
			this.Prompt = string(data)
			rendered := this.PromptStyle.Render(this.Prompt)
			this.ParentOut.Write([]byte(rendered))
		} else if hasCarriageReturn {
			this.ChildIn.Write(data)
		} else if data[0] == '\t' { // user is asking to fill in an autosuggest
			if this.LastAutosuggest != "" {
				this.ChildIn.Write([]byte(this.LastAutosuggest))
			} else {
				// no last autosuggest found, just forward the tab
				this.ChildIn.Write(data)
			}
		} else {
			log.Printf("State change: normal -> shell")
			this.State = stateShell
			this.History.NewBlock()
			this.ChildIn.Write(data)
			this.Command = NewShellBuffer()
			this.Command.Write(string(data))
		}

	case statePrompting:
		// check if the input contains a newline
		toAdd := data
		if hasCarriageReturn {
			toAdd = data[:bytes.Index(data, []byte{'\r'})]
			this.State = stateNormal
			log.Printf("State change: prompting -> normal")
		}
		this.Prompt += string(toAdd)
		rendered := this.Butterfish.Config.Styles.Question.Render(string(toAdd))
		this.ParentOut.Write([]byte(rendered))

		// Submit this prompt if we just switched back into stateNormal
		if this.State == stateNormal {
			//parentOut.Write([]byte("\n\rPrompting: " + prompt + "\n\r"))
			log.Printf("Prompting: %s", this.Prompt)
			this.ParentOut.Write([]byte("\n\r"))

			historyBlocks := this.History.GetLastNBytes(3000)
			request := &util.CompletionRequest{
				Ctx:           ctx,
				Prompt:        this.Prompt,
				Model:         BestCompletionModel,
				MaxTokens:     512,
				Temperature:   0.7,
				HistoryBlocks: historyBlocks,
			}

			//dump, _ := json.Marshal(historyBlocks)
			//fmt.Fprintf(parentOut, "History: %s\n\r", dump)
			output, err := this.Butterfish.LLMClient.CompletionStream(request, this.PromptAnswerWriter)
			if err != nil {
				log.Printf("Error: %s", err)
			}

			this.History.Add(historyTypePrompt, this.Prompt)
			this.History.Add(historyTypeLLMOutput, output)

			this.ChildIn.Write([]byte("\n"))
			this.Prompt = ""
			this.RequestAutosuggest(true)
		}

	case stateShell:
		if hasCarriageReturn { // user is submitting a command
			this.ClearAutosuggest()

			this.State = stateNormal
			this.ChildIn.Write(data)
			this.Command = NewShellBuffer()
			this.History.NewBlock()
			log.Printf("State change: shell -> normal")
		} else if data[0] == '\t' { // user is asking to fill in an autosuggest
			// Tab was pressed, fill in lastAutosuggest
			if this.LastAutosuggest != "" {
				this.ChildIn.Write([]byte(this.LastAutosuggest))
			} else {
				// no last autosuggest found, just forward the tab
				this.ChildIn.Write(data)
			}
		} else { // otherwise user is typing a command
			this.ChildIn.Write(data)
			this.Command.Write(string(data))
			this.RefreshAutosuggest(data)
		}

	default:
		panic("Unknown state")
	}
}

// Update autosuggest when we receive new data
func (this *ShellState) RefreshAutosuggest(newData []byte) {
	// check if data is a prefix of lastautosuggest
	if bytes.HasPrefix([]byte(this.LastAutosuggest), newData) {
		this.LastAutosuggest = this.LastAutosuggest[len(newData):]
		return
	}

	// otherwise, clear the autosuggest
	this.ClearAutosuggest()

	// and request a new one
	if this.State == stateShell {
		this.RequestAutosuggest(false)
	}
}

// Clear out the grayed out autosuggest text we wrote previously
func (this *ShellState) ClearAutosuggest() {
	if this.LastAutosuggest == "" {
		// there wasn't actually a last autosuggest, so nothing to clear
		return
	}

	// TODO special case when the added character is the same as the lastAutosuggest
	// clear out the last autosuggest
	for i := 0; i < len(this.LastAutosuggest); i++ {
		this.ParentOut.Write([]byte(" "))
	}
	for i := 0; i < len(this.LastAutosuggest); i++ {
		this.ParentOut.Write(cursorLeft)
	}
	this.LastAutosuggest = ""
}

var autosuggestDelay = 100 * time.Millisecond

func (this *ShellState) RequestAutosuggest(noDelay bool) {
	if this.AutosuggestCancel != nil {
		// clear out a previous request
		this.AutosuggestCancel()
	}
	this.AutosuggestCtx, this.AutosuggestCancel = context.WithCancel(context.Background())
	historyBlocks := HistoryBlocksToString(this.History.GetLastNBytes(2000))
	//this.History.LogRecentHistory()

	var delay time.Duration
	if !noDelay {
		delay = autosuggestDelay
	}

	go RequestCancelableAutosuggest(
		this.AutosuggestCtx, delay, this.Command.String(),
		historyBlocks, this.Butterfish.LLMClient, this.AutosuggestChan)
}

func RequestCancelableAutosuggest(
	ctx context.Context,
	delay time.Duration,
	currCommand string,
	historyText string,
	llmClient LLM,
	autosuggestChan chan<- *AutosuggestResult) {

	if delay > 0 {
		time.Sleep(delay)
	}
	if ctx.Err() != nil {
		return
	}
	var prompt string

	if len(currCommand) == 0 {
		prompt = fmt.Sprintf(`The user is using a Unix shell but hasn't yet entered anything. Suggest a unix command based on previous assistant output like an example. If the user has entered a command recently which failed, suggest a fixed version of that command. Respond with only the shell command, do not add comments or quotations. Here is the recent history:
'''
%s
'''`, historyText)
	} else {
		prompt = fmt.Sprintf(`The user is asking for an autocomplete suggestion for this Unix shell command, respond with only the suggested command, which should include the original command text, do not add comments or quotations. Here is some recent context and history:
'''
%s
'''.
If a command has resulted in an error, avoid that. This is the start of the command: '%s'.`, historyText, currCommand)
	}

	request := &util.CompletionRequest{
		Ctx:         ctx,
		Prompt:      prompt,
		Model:       BestAutosuggestModel,
		MaxTokens:   256,
		Temperature: 0.7,
		//HistoryBlocks: historyBlocks,
	}

	//log.Printf("Autosuggesting: %s %x\n%s", currCommand, []byte(currCommand), request.Prompt)

	output, err := llmClient.Completion(request)
	if err != nil {
		return
	}

	autoSuggest := &AutosuggestResult{
		Command:    currCommand,
		Suggestion: output,
	}
	autosuggestChan <- autoSuggest
}

var cursorLeft []byte = []byte{27, 91, 68}

func (this *ButterfishCtx) ShellMultiplexer(
	childIn io.Writer, childOut io.Reader,
	parentIn io.Reader, parentOut io.Writer) {
	childOutReader := make(chan *byteMsg)
	parentInReader := make(chan *byteMsg)

	go readerToChannel(childOut, childOutReader)
	go readerToChannel(parentIn, parentInReader)
	log.Printf("Starting shell multiplexer")

	promptOutputWriter := util.NewStyledWriter(parentOut, this.Config.Styles.Answer)
	cleanedWriter := util.NewReplaceWriter(promptOutputWriter, "\n", "\r\n")

	shellState := &ShellState{
		Butterfish:         this,
		ParentOut:          parentOut,
		ChildIn:            childIn,
		State:              stateNormal,
		ChildOutReader:     childOutReader,
		ParentInReader:     parentInReader,
		AutosuggestChan:    make(chan *AutosuggestResult),
		History:            NewShellHistory(),
		PromptAnswerWriter: cleanedWriter,
		PromptStyle:        this.Config.Styles.Question,
		AutosuggestStyle:   this.Config.Styles.Grey,
	}

	shellState.Mux()
}

func RunConsoleClient(ctx context.Context, args []string) error {
	client, err := runIPCClient(ctx)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)

	return wrapCommand(ctx, cancel, args, client) // this is blocking
}

func RunConsole(ctx context.Context, config *ButterfishConfig) error {
	ctx, cancel := context.WithCancel(ctx)

	// initialize console UI
	consoleCommand := make(chan string)
	cmdCallback := func(cmd string) {
		consoleCommand <- cmd
	}
	exitCallback := func() {
		cancel()
	}
	configCallback := func(model console.ConsoleModel) console.ConsoleModel {
		model.SetStyles(config.Styles.Prompt, config.Styles.Question)
		return model
	}
	cons := console.NewConsoleProgram(configCallback, cmdCallback, exitCallback)

	llmClient, err := initLLM(config)
	if err != nil {
		return err
	}

	clientController := RunIPCServer(ctx, cons)

	promptLibrary, err := initPromptLibrary(config)
	if err != nil {
		return err
	}

	butterfishCtx := ButterfishCtx{
		Ctx:              ctx,
		Cancel:           cancel,
		PromptLibrary:    promptLibrary,
		InConsoleMode:    true,
		Config:           config,
		LLMClient:        llmClient,
		Out:              cons,
		ConsoleCmdChan:   consoleCommand,
		ClientController: clientController,
	}

	// this is blocking
	butterfishCtx.serverMultiplexer()

	return nil
}

func initLLM(config *ButterfishConfig) (LLM, error) {
	if config.OpenAIToken == "" && config.LLMClient != nil {
		return nil, errors.New("Must provide either an OpenAI Token or an LLM client.")
	} else if config.OpenAIToken != "" && config.LLMClient != nil {
		return nil, errors.New("Must provide either an OpenAI Token or an LLM client, not both.")
	} else if config.OpenAIToken != "" {
		verboseWriter := util.NewStyledWriter(os.Stdout, config.Styles.Grey)
		return NewGPT(config.OpenAIToken, config.Verbose, verboseWriter), nil
	} else {
		return config.LLMClient, nil
	}
}

func initPromptLibrary(config *ButterfishConfig) (PromptLibrary, error) {
	verboseWriter := util.NewStyledWriter(os.Stdout, config.Styles.Grey)

	if config.PromptLibrary != nil {
		return config.PromptLibrary, nil
	}

	promptPath, err := homedir.Expand(config.PromptLibraryPath)
	if err != nil {
		return nil, err
	}

	return NewDiskPromptLibrary(promptPath, config.Verbose, verboseWriter)
}

func NewButterfish(ctx context.Context, config *ButterfishConfig) (*ButterfishCtx, error) {
	llmClient, err := initLLM(config)
	if err != nil {
		return nil, err
	}

	promptLibrary, err := initPromptLibrary(config)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	butterfishCtx := &ButterfishCtx{
		Ctx:           ctx,
		Cancel:        cancel,
		PromptLibrary: promptLibrary,
		InConsoleMode: false,
		Config:        config,
		LLMClient:     llmClient,
		Out:           os.Stdout,
	}

	return butterfishCtx, nil
}
