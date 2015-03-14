// Initial code inspiration text/template/parse, which is licensed as:

// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Imitation is the sincerest form of flattery.
// (c) Ben Nagy 2015

package main

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Pos int

// item represents a token or text string returned from the scanner.
type item struct {
	typ itemType // The type of this item.
	pos Pos      // The starting position, in bytes, of this item in the input string.
	val string   // The value of this item.
}

// itemType identifies the type of lex items.
type itemType int

const (
	itemError itemType = iota // error occurred; value is text of error
	itemEOF
	itemNumber    // PDF Number 7.3.3
	itemSpace     // run of space characters 7.2.2 Table 1
	itemLeftDict  // Just the << token
	itemRightDict // >> token
	itemLeftArray
	itemRightArray
	itemStreamBody // raw contents of a stream
	itemString     // PDF Literal String 7.3.4.2
	itemHexString  // PDF Hex String 7.3.4.3
	itemComment    // 7.2.3
	itemName       // PDF Name Object 7.3.5
	itemWord       // catchall for an unrecognised blob of alnums
	// Keywords appear after all the rest.
	itemKeyword // used only to delimit the keywords
	itemObj     // just the obj and endobj markers
	itemEndObj
	itemStream // just the markers
	itemEndStream
	itemTrailer
	itemXref
	itemStartXref
	itemTrue  // not really keywords, they're actually types of
	itemFalse // PDF Basic Object, but this is cleaner 7.3.2
	itemNull
)

// If they need to be used directly in code then a constant string is easiest
const (
	leftDict    = "<<"
	rightDict   = ">>"
	leftStream  = "stream"
	rightStream = "endstream"
)

// keytoks maps special strings to itemTypes
var keytoks = map[string]itemType{
	"obj":       itemObj,
	"endobj":    itemEndObj,
	leftStream:  itemStream,
	rightStream: itemEndStream,
	"trailer":   itemTrailer,
	"xref":      itemXref,
	"startxref": itemStartXref,
	"true":      itemTrue,
	"false":     itemFalse,
	"null":      itemNull,
}

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the scanner.
type lexer struct {
	name       string    // the name of the input; used only for error reports
	input      string    // the string being scanned
	state      stateFn   // the next lexing function to enter
	pos        Pos       // current position in the input
	start      Pos       // start position of this item
	width      Pos       // width of last rune read from input
	lastPos    Pos       // position of most recent item returned by nextItem
	items      chan item // channel of scanned items
	arrayDepth int       // nesting depth of [], <<>>
	dictDepth  int
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = Pos(w)
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Must only be called once per call of next.
func (l *lexer) backup() {
	l.pos -= l.width
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	l.items <- item{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
}

// lineNumber reports which line we're on, based on the position of
// the previous item returned by nextItem. Doing it this way
// means we don't have to worry about peek double counting.
func (l *lexer) lineNumber() int {
	return 1 + strings.Count(l.input[:l.lastPos], "\n")
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	item := <-l.items
	l.lastPos = item.pos
	return item
}

// lex creates a new scanner for the input string.
func lex(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for l.state = lexDefault; l.state != nil; {
		l.state = l.state(l)
	}
}

// state functions

// lexDefault is the main lexing state. The rules here work for the root
// namespace, as well as inside dicts <<>> and arrays [].
func lexDefault(l *lexer) stateFn {
	switch r := l.next(); {
	case unicode.IsSpace(r):
		return lexSpace
	case r == '/':
		return lexName
	case r == '+' || r == '-' || r == '.' || ('0' <= r && r <= '9'):
		l.backup()
		return lexNumber
		// strings and hex objects have stricter rules
	case isAlphaNumeric(r):
		return lexWord
	case r == '(':
		return lexStringObj
	// dicts and arrays can nest arbitrarily deeply. We're not a parser, but
	// let's just sanity check termination.
	case r == '<':
		if l.peek() == '<' {
			l.backup()
			l.dictDepth++
			return lexLeftDict
		}
		return lexHexObj
	// Arrays are just collections of objects, so all these default rules are still fine
	case r == '[':
		l.emit(itemLeftArray)
		l.arrayDepth++
		return lexDefault
	case r == ']':
		l.arrayDepth--
		if l.arrayDepth < 0 {
			return l.errorf("unexexpected array terminator")
		}
		l.emit(itemRightArray)
		return lexDefault
	case r == '%':
		return lexComment
	case r == '>':
		if l.peek() == '>' {
			l.dictDepth--
			if l.dictDepth < 0 {
				return l.errorf("unexexpected dict terminator")
			}
			l.backup()
			return lexRightDict
		}
		// '>' as part of a hex object should have been consumed in lexHex, so
		// a stray '>' in this state is not valid.
		fallthrough
	case r == eof:
		if l.arrayDepth > 0 {
			return l.errorf("unterminated array")
		}
		if l.dictDepth > 0 {
			return l.errorf("unterminated dict")
		}
		l.emit(itemEOF)
		return nil

	default:
		return l.errorf("illegal character: %#U", r)
	}
	return lexDefault
}

// lexStream quickly skips over all the contents of PDF stream objects. The
// 'stream' header has already been consumed and emitted in lexWord.
func lexStream(l *lexer) stateFn {
	i := strings.Index(l.input[l.pos:], rightStream)
	if i < 0 {
		return l.errorf("unclosed stream")
	}
	l.pos += Pos(i)
	l.emit(itemStreamBody)
	l.pos += Pos(len(rightStream))
	l.emit(itemEndStream)
	return lexDefault
}

// lexLeftDict scans the left delimiter, which is known to be present.
func lexLeftDict(l *lexer) stateFn {
	l.pos += Pos(len(leftDict))
	l.emit(itemLeftDict)
	return lexDefault
}

// lexComment lexes a PDF comment from a comment marker % to the next EOL
// marker. However, '\r\n' (specifically) is treated as one EOL marker. Some
// comments such as %%EOF and %PDF-1.7 are special to reader software, but
// that's parser business.
// cf PDF3200_2008.pdf 7.2.2
func lexComment(l *lexer) stateFn {

	var r rune
	for !isEndOfLine(l.peek()) {
		r = l.next()
	}

	// any single EOL marker has been consumed above. Check for CRLF.
	if r == '\r' {
		l.accept("\n")
	}

	l.emit(itemComment)
	return lexDefault
}

// lexRightDict scans the right delimiter, which is known to be present.
func lexRightDict(l *lexer) stateFn {
	l.pos += Pos(len(rightDict))
	l.emit(itemRightDict)
	return lexDefault
}

// lexName scans a PDF Name object, which is a SOLIDUS (lol) '/' followed by a
// run of non-special characters. Unprintable ASCII must be escaped with '#XX'
// codes.
// cf PDF3200_2008.pdf 7.3.5
func lexName(l *lexer) stateFn {
	for {
		switch r := l.next(); {
		case isDelim(r) || unicode.IsSpace(r) || r == eof:
			l.backup()
			l.emit(itemName)
			return lexDefault
		case 0x20 < r && r < 0x7f:
			break
		default:
			return l.errorf("illegal character in name: %#U", r)
		}
	}
}

// lexStringObj scans a PDF String object which is any collection of bytes
// enclosed in parens (). Strings can contain balanced parens, or unbalanced
// parens that are escaped with '\'. There are some other rules about what to
// do with parsing linebreaks and escaped special chars, but that's above our
// pay grade here.
// cf PDF3200_2008.pdf 7.3.4.2
func lexStringObj(l *lexer) stateFn {
	balance := 1
	for {
		switch r := l.next(); {
		case r == '\\':
			// escaped parens don't count towards balance
			l.accept("()")
		case r == '(':
			balance++
		case r == ')':
			balance--
			if balance <= 0 {
				l.emit(itemString)
				return lexDefault
			}
		case r == eof:
			return l.errorf("unterminated string object")
		default:
		}
	}
}

// lexHexObj scans a hex string, which is any number of hexadecimal characters
// or whitespace enclosed by '<' '>'. The '<' rune has already been consumed.
// cf PDF3200_2008.pdf 7.3.4.3
func lexHexObj(l *lexer) stateFn {
	digits := "0123456789abcdefABCDEF"
	for {
		switch r := l.next(); {
		case strings.IndexRune(digits, r) >= 0 || unicode.IsSpace(r):
			//
		case r == '>':
			l.emit(itemHexString)
			return lexDefault
		case r == eof:
			return l.errorf("unterminated hexstring")
		default:
			return l.errorf("illegal character in hexstring: %#U", r)
		}
	}
}

// lexSpace scans a run of space characters one of which has already been seen.
// cf PDF3200_2008.pdf 7.2.2
func lexSpace(l *lexer) stateFn {
	// This is more permissive than the spec, which doesn't mention U+0085
	// (NEL), U+00A0 (NBSP)
	for unicode.IsSpace(l.peek()) {
		l.next()
	}
	l.emit(itemSpace)
	return lexDefault
}

// lexWord scans a run of basic alnums, one of which has already been seen. It
// will emit known tokens as their special types, call new state functions for
// types that require special lexing, and, failing that, emit the run as a
// catchall itemWord and then return to lexDefault
func lexWord(l *lexer) stateFn {

	for isAlphaNumeric(l.peek()) {
		l.next()
	}

	tok, found := keytoks[l.input[l.start:l.pos]]
	if found {
		// known token type, emit it
		l.emit(tok)
		switch tok {
		case itemStream:
			return lexStream
		default:
			return lexDefault
		}
	}

	l.emit(itemWord)
	return lexDefault
}

// lexNumber scans a decimal or real number
// cf PDF3200_2008.pdf 7.3.3
func lexNumber(l *lexer) stateFn {
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	l.emit(itemNumber)
	return lexDefault
}

func (l *lexer) scanNumber() bool {
	// Optional leading sign.
	l.accept("+-")
	digits := "0123456789"
	l.acceptRun(digits)
	if l.accept(".") {
		l.acceptRun(digits)
	}
	// Next thing must be a delimeter, space char or eof
	if isDelim(l.peek()) || unicode.IsSpace(l.peek()) || l.peek() == eof {
		return true
	}
	l.next()
	return false
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

// isDelim reports whether r is one of the 10 reserved PDF delimiter characters
// cf PDF3200_2008.pdf 7.2.2
func isDelim(r rune) bool {
	return strings.IndexRune("[]{}()<>/%", r) >= 0
}

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
