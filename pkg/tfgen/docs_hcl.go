// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tfgen

import (
	"github.com/hashicorp/hcl/hcl/scanner"
	"github.com/hashicorp/hcl/hcl/token"
	"github.com/hashicorp/hcl2/hclparse"
)

// fixHcl attempts to fix certain simple syntactical errors in a particular piece of HCL source text.
//
// For reference, here is the HCL grammar in ~EBNF:
//
//     file := objectList EOF
//
//     objectList := { objectItem }
//
//     objectItem := ( assignmentProperty | objectProperty )
//
//     assignmentProperty := ( IDENT | STRING ) '=' value
//
//     objectProperty := ( IDENT | STRING ) { ( IDENT | STRING ) } objectValue
//
//     value := literalValue | objectValue | listValue
//
//     literalValue := NUMBER | FLOAT | BOOL | STRING | HEREDOC
//
//     objectValue := '{' objectList '}'
//
//     listValue := '[' [ value { ',' value } ] ']'
//
// We want to fix the following errors:
// - missing value in an assignmentProperty
// - missing objectValue in an objectProperty
// - missing values in a list
//
// We do not need to fix;
// - unbalanced braces
// - unbalanced brackets
// - bad assignmentProperty keys
// - etc.
//
// All of the problems we want to fix can be addressed by inserting artificial tokens into the token stream.
//
// NOTE: some valid HCL2 looks like invalid HCL. We try to catch these cases by checking for valid HCL2 first.
func fixHcl(hcl string) (string, bool) {
	input := []byte(hcl)

	if _, diags := hclparse.NewParser().ParseHCL(input, "main.tf"); len(diags) == 0 {
		return "", false
	}

	h := &hclFixer{
		input:   input,
		scanner: scanner.New(input),
	}
	h.scanner.Error = func(_ token.Pos, _ string) {}

	if !h.file() {
		return "", false
	}

	return string(h.output), true
}

// hclFixer is a utility type that is used by fixHcl.
type hclFixer struct {
	input  []byte // the input source text
	output []byte // the output source text

	scanner *scanner.Scanner // the scanner that lexes the input source text
	next    *token.Token     // the next token to return

	flushed int // the number of bytes of source text that have been processed
}

// peek returns the next token in the input text, but does not advance the current position or copy the token to the
// output buffer.
func (h *hclFixer) peek() token.Token {
	// If the buffer is empty, grab the next token from the scanner.
	if h.next == nil {
		t := h.scanner.Scan()

		// Chew through comments. We save the offset of the first comment token and stamp it onto the first non-comment
		// token we see so that patches (if any) will occur before comments. For example, given the following:
		//
		//     resource "aws_s3_bucket" "foo" {
		//         name = # put a name here
		//     }
		//
		// If we patch the missing property value, we want the patch to occur before "# put a name here". If we simply
		// skip the comment without recording its offset, we would end up patching before the '}'.
		offset := t.Pos.Offset
		for t.Type == token.COMMENT {
			t = h.scanner.Scan()
		}
		t.Pos.Offset = offset

		// If this token is of a type we never expect to appear in source code, treat it as an illegal token. The
		// patcher chews through illegal tokens after applying a patch so that we can handle code like this:
		//
		//     resource "aws_s3_bucket" "foo" {
		//         name = ... # put a name here
		//     }
		//
		// In this case, we want the patcher to be able to add the missing property before the ellipsis and then drop
		// the ellipsis from the output.
		switch t.Type {
		case token.ADD, token.SUB, token.PERIOD:
			t.Type = token.ILLEGAL
		}

		// Finally, stick the new token into the buffer.
		h.next = &t
	}
	return *h.next
}

// scan returns the next token in the input text, copies it to the output buffer, and advances the current position.
func (h *hclFixer) scan() token.Token {
	// Pull the current token out of the buffer.
	t := h.peek()
	h.next = nil

	// Now refill the buffer so we can calculate the length of the current token.
	h.peek()

	// If the current token is not ILLEGAL, flush it to the output.
	n := h.next.Pos.Offset - h.flushed
	if t.Type != token.ILLEGAL {
		h.output = append(h.output, h.input[:n]...)
	}
	h.input = h.input[n:]
	h.flushed += n

	return t
}

// patch inserts the given patch into the output stream and then chews through any illegal characters in the source
// test. This is the primary mechanism for correcting errors in the input.
func (h *hclFixer) patch(p string) {
	h.output = append(h.output, []byte(p)...)

	// Chomp through any illegal tokens to accommodate truly strange input
	for h.peek().Type == token.ILLEGAL {
		h.scan()
	}
}

// file parses an HCL file production.
func (h *hclFixer) file() bool {
	if !h.objectList() {
		return false
	}
	return h.scan().Type == token.EOF
}

// objectList parses an HCL objectList production.
func (h *hclFixer) objectList() bool {
	for {
		// peek should be an IDENT or a STRING
		next := h.peek().Type
		if next != token.IDENT && next != token.STRING {
			return true
		}

		if !h.objectItem() {
			return false
		}
	}
}

// objectItem parses an HCL objectItem production.
func (h *hclFixer) objectItem() bool {
	// The next token should be an IDENT or a STRING
	keyPart := h.scan().Type
	if keyPart != token.IDENT && keyPart != token.STRING {
		return false
	}

	// If the next token is an '=', parse an assignmentProperty.
	next := h.peek().Type
	if next == token.ASSIGN {
		return h.assignmentProperty()
	}

	// Otherwise, continue chomping up strings or identifiers until we hit something else. If the thing we hit is not
	// an LBRACE, we have a missing object property and will fill one in.
	for next == token.IDENT || next == token.STRING {
		h.scan()
		next = h.peek().Type
	}

	// If the next token after the key is an LBRACE, parse an objectValue.
	if next == token.LBRACE {
		return h.objectValue()
	}

	// Otherwise, we have a missing objectValue. Synthesize one now and carry on.
	h.patch("{}")
	return true
}

// assignmentProperty parses an HCL assignmentProperty production.
func (h *hclFixer) assignmentProperty() bool {
	if h.scan().Type != token.ASSIGN {
		return false
	}

	switch h.peek().Type {
	case token.NUMBER, token.FLOAT, token.BOOL, token.STRING, token.HEREDOC:
		// Eat the literalValue and return true.
		h.scan()
		return true

	case token.LBRACE:
		// Parse an objectValue
		return h.objectValue()

	case token.LBRACK:
		// Parse a listValue
		return h.listValue()

	default:
		// We have a missing value. Sythesize one here and carry on.
		h.patch(`""`)
		return true
	}
}

// objectValue parses an HCL objectValue production.
func (h *hclFixer) objectValue() bool {
	return h.scan().Type == token.LBRACE && h.objectList() && h.scan().Type == token.RBRACE
}

// listValue parses an HCL listValue production.
func (h *hclFixer) listValue() bool {
	if h.scan().Type != token.LBRACK {
		return false
	}

	for i := 0; ; i++ {
		if i != 0 {
			switch h.peek().Type {
			case token.COMMA:
				h.scan()
			case token.RBRACK:
				h.scan()
				return true
			default:
				return false
			}
		}

		switch h.peek().Type {
		case token.NUMBER, token.FLOAT, token.BOOL, token.STRING, token.HEREDOC:
			// Eat the literalValue and continue
			h.scan()

		case token.LBRACE:
			// Parse an objectValue
			if !h.objectValue() {
				return false
			}

		case token.LBRACK:
			// Parse a listValue
			if !h.listValue() {
				return false
			}

		case token.RBRACK:
			h.scan()
			return true

		case token.COMMA:
			// We have a missing value. Sythesize one here and carry on.
			h.patch(`""`)

		default:
			return false
		}
	}
}
