// Diff-inspector demonstrates the public agent protocol without a model or host APIs.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/hyaxon/giad/pkg/protocol"
)

func manifest(command string) protocol.Manifest {
	return protocol.Manifest{
		APIVersion: protocol.Version, Name: "diff-inspector", Version: "0.1.0",
		Entrypoint:    protocol.Entrypoint{Command: command, Args: []string{"--stdio"}},
		Capabilities:  protocol.Capabilities{Required: []string{"git.diff", "repository.instructions"}, Optional: []string{}},
		ModelProfiles: []string{},
	}
}

func main() {
	showManifest := flag.Bool("manifest", false, "Print a manifest using this executable's absolute path")
	container := flag.Bool("container", false, "Use the installed image's executable path in the manifest")
	stdio := flag.Bool("stdio", false, "Run the GIAD agent protocol over stdin/stdout")
	flag.Parse()
	if flag.NArg() != 0 || *showManifest == *stdio || (*container && !*showManifest) {
		fmt.Fprintln(os.Stderr, "use exactly one of --manifest or --stdio")
		os.Exit(2)
	}
	var err error
	if *showManifest {
		var command string
		command, err = os.Executable()
		if *container {
			command = "/agent/diff-inspector"
		}
		if err == nil {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			err = encoder.Encode(manifest(command))
		}
	} else {
		err = run(os.Stdin, os.Stdout)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "diff-inspector:", err)
		os.Exit(1)
	}
}

type peer struct {
	input  *bufio.Scanner
	output *json.Encoder
	calls  int
}

func (p *peer) receive() (protocol.Frame, error) {
	if !p.input.Scan() {
		if err := p.input.Err(); err != nil {
			return protocol.Frame{}, err
		}
		return protocol.Frame{}, io.ErrUnexpectedEOF
	}
	var frame protocol.Frame
	if err := json.Unmarshal(p.input.Bytes(), &frame); err != nil {
		return frame, err
	}
	if frame.APIVersion != protocol.Version {
		return frame, fmt.Errorf("unsupported host apiVersion %q; expected %q", frame.APIVersion, protocol.Version)
	}
	return frame, nil
}

func (p *peer) request(method string, params, result any) error {
	data, err := json.Marshal(params)
	if err != nil {
		return err
	}
	p.calls++
	id := strconv.Itoa(p.calls)
	if err := p.output.Encode(protocol.Frame{APIVersion: protocol.Version, ID: id, Method: method, Params: data}); err != nil {
		return err
	}
	reply, err := p.receive()
	if err != nil {
		return err
	}
	if reply.ID != id || reply.Method != "" {
		return errors.New("unexpected host response")
	}
	if reply.Error != "" {
		return fmt.Errorf("%s: %s", method, reply.Error)
	}
	if reply.Result == nil {
		return errors.New("host response has no result")
	}
	return json.Unmarshal(reply.Result, result)
}

func run(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), protocol.MaxMessageBytes)
	p := peer{input: scanner, output: json.NewEncoder(output)}
	start, err := p.receive()
	if err != nil {
		return err
	}
	if start.Method != "review.start" || start.ID != "" {
		return errors.New("expected review.start notification")
	}
	var job protocol.Job
	if err := json.Unmarshal(start.Params, &job); err != nil {
		return err
	}
	// This tutorial reads the diff but makes no judgment about repository policy.
	var diff struct {
		Text         string
		Truncated    bool
		SkippedFiles int
	}
	if err := p.request("git.diff", struct{}{}, &diff); err != nil {
		return err
	}
	report := protocol.Report{
		Summary:     fmt.Sprintf("Retrieved %d bytes of diff for %d changed files.", len(diff.Text), len(job.ChangedFiles)),
		Limitations: "Protocol example only: no defect analysis or tests were performed; repository guidance was not evaluated.",
		Findings:    []protocol.Finding{},
	}
	if diff.Truncated {
		report.Limitations += " The diff was truncated."
	}
	var accepted struct {
		Accepted bool `json:"accepted"`
	}
	if err := p.request("review.finish", report, &accepted); err != nil {
		return err
	}
	if !accepted.Accepted {
		return errors.New("host did not accept the report")
	}
	return nil
}
