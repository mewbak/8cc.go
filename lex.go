package main

import (
	"strconv"
)

const BUFLEN = 256

const (
	TTYPE_IDENT int = iota
	TTYPE_PUNCT
	TTYPE_INT
	TTYPE_CHAR
	TTYPE_STRING
)

type Token struct {
	typ int
	v   struct { // wanna be Union
		ival  int
		sval  []byte
		punct byte
		c     byte
	}
}

var ungotten *Token

func make_ident(s []byte) *Token {
	r := &Token{}
	r.typ = TTYPE_IDENT
	r.v.sval = s
	return r
}

func make_strtok(s []byte) *Token {
	r := &Token{}
	r.typ = TTYPE_STRING
	r.v.sval = s
	return r
}

func make_punct(punct byte) *Token {
	r := &Token{}
	r.typ = TTYPE_PUNCT
	r.v.punct = punct
	return r
}

func make_int(n int) *Token {
	r := &Token{}
	r.typ = TTYPE_INT
	r.v.ival = n
	return r
}

func make_char(c byte) *Token {
	r := &Token{}
	r.typ = TTYPE_CHAR
	r.v.c = c
	return r
}

func getc_nonspace() (byte, error) {
	var c byte
	var err error
	for {
		c, err = getc(stdin)
		if err != nil {
			break
		}
		if isspace(c) || c == byte('\n') || c == byte('\r') {
			continue
		}
		return c,nil
	}
	return 0,err
}

func read_number(c byte) *Token {
	n := int(c - '0')
	for {
		c, _ := getc(stdin)
		if !isdigit(c) {
			ungetc(c, stdin)
			return make_int(n)
		}
		n = n*10 + int(c-'0')
	}
}

func read_char() *Token {
	c, err := getc(stdin)
	if err != nil {
		_error("Unterminated char")
	}
	if c == '\\' {
		c, err = getc(stdin)
		if err != nil {
			_error("Unterminated char")
		}
	}

	c2, err := getc(stdin)
	if err != nil {
		_error("Unterminated char")
	}
	if c2 != '\'' {
		_error("Malformed char constant")
	}

	return make_char(c)
}

func read_string() *Token {
	buf := make([]byte, BUFLEN)
	i := 0
	for {
		c, err := getc(stdin)
		if err != nil {
			_error("Unterminated string")
		}
		if c == '"' {
			break
		}
		if c == '\\' {
			c, err = getc(stdin)
			if err != nil {
				_error("Unterminated \\")
			}
		}
		buf[i] = c
		i++
		if i == BUFLEN-1 {
			_error("String too long")
		}
	}
	buf[i] = 0
	return make_strtok(buf)
}

func read_ident(c byte) *Token {
	buf := make([]byte, BUFLEN)
	buf[0] = c
	i := 1
	for {
		c2, _ := getc(stdin)
		if !isalnum(c2) {
			ungetc(c2, stdin)
			break
		}
		buf[i] = c2
		i++
		if i == (BUFLEN - 1) {
			_error("Identifier too long")
		}
	}
	buf[i] = 0
	return make_ident(buf)
}

func read_token_init() *Token {
	c, err := getc_nonspace()
	if err != nil {
		// EOF
		return nil
	}

	// TODO use switch syntax
	if '0' <= c && c <= '9' {
		return read_number(c)
	}
	if c == '"' {
		return read_string()
	}
	if c == '\'' {
		return read_char()
	}
	if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '_' {
		return read_ident(c)
	}
	if c == '/' || c == '=' || c == '*' ||
		c == '+' || c == '-' || c == '(' ||
		c == ')' || c == ',' || c == ';' {
		return make_punct(c)
	}
	_error("Don't know how to handle '%c'", c)
	return nil
}

func token_to_string(tok *Token) []byte {
	switch tok.typ {
	case TTYPE_IDENT:
		return tok.v.sval
	case TTYPE_PUNCT:
		return []byte{tok.v.punct, 0}
	case TTYPE_CHAR:
		return []byte{tok.v.c, 0}
	case TTYPE_INT:
		return []byte(strconv.Itoa(tok.v.ival))
	case TTYPE_STRING:
		return tok.v.sval
	default:
		_error("internal error: unknown token type: %d", tok.typ)
	}
	return nil
}

func is_punct(tok *Token, c byte) bool {
	if tok == nil {
		_error("Token is null")
	}
	return tok.typ == TTYPE_PUNCT && tok.v.punct == c
}

func unget_token(tok *Token) {
	if ungotten != nil {
		_error("Push back buffer is already full")
	}
	ungotten = tok
}

func peek_token() *Token {
	tok := read_token()
	unget_token(tok)
	return tok
}

func read_token() *Token {
	if ungotten != nil {
		tok := ungotten
		ungotten = nil
		return tok
	}

	return read_token_init()
}