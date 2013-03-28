package canonical

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
)

var (
	lowerCase        = []byte("abcdefghijklmnopqrstuvwxyz")
	upperCase        = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	decimalDigit     = []byte("0123456789")
	alpha            = append(lowerCase, upperCase...)
	hexadecimalDigit = append(decimalDigit, []byte("abcdefABCDEF")...)
	octalDigit       = []byte("01234567")
	simplePunc       = []byte("-./_:*+=")
	whitespaceChar   = []byte(" \t\r\n")
	base64Char       = append(alpha, append(decimalDigit, []byte("+/=")...)...)
	tokenChar        = append(alpha, append(decimalDigit, simplePunc...)...)
	base64Decoder    = base64.StdEncoding
	stringChar       = append(tokenChar, append(hexadecimalDigit, []byte("\"|#")...)...)
)

type Sexpr interface {
	ToAdvanced() string
	toAdvanced(*bytes.Buffer)
	ToCanonical() []byte
	toCanonical(*bytes.Buffer)
}

type List []Sexpr

type Atom struct {
	DisplayHint []byte
	Value       []byte
}

func (a Atom) ToCanonical() []byte {
	buf := bytes.NewBuffer(nil)
	a.toCanonical(buf)
	return buf.Bytes()
}

func (a Atom) toCanonical(buf *bytes.Buffer) {
	if a.DisplayHint != nil && len(a.DisplayHint) > 0 {
		buf.WriteString("[" + strconv.Itoa(len(a.DisplayHint)) + ":")
		buf.Write(a.DisplayHint)
		buf.WriteString("]")
	}
	buf.WriteString(strconv.Itoa(len(a.Value)) + ":")
	buf.Write(a.Value)
}

func (a Atom) ToAdvanced() string {
	buf := bytes.NewBuffer(nil)
	a.toAdvanced(buf)
	return buf.String()
}

func (a Atom) toAdvanced(buf *bytes.Buffer) {
	buf.WriteString(strconv.Itoa(len(a.Value)) + ":")
	buf.Write(a.Value)
}

func (l List) ToCanonical() []byte {
	buf := bytes.NewBuffer(nil)
	l.toCanonical(buf)
	return buf.Bytes()
}

func (l List) toCanonical(buf *bytes.Buffer) {
	buf.WriteString("(")
	for _, datum := range l {
		datum.toCanonical(buf)
	}
	buf.WriteString(")")
}

func (l List) ToAdvanced() string {
	buf := bytes.NewBuffer(nil)
	l.toAdvanced(buf)
	return buf.String()
}

func (l List) toAdvanced(buf *bytes.Buffer) {
	buf.WriteString("(")
	for _, datum := range l {
		datum.toAdvanced(buf)
	}
	buf.WriteString(")")
}

func ParseBytes(bytes []byte) (sexpr Sexpr, rest []byte, err error) {
	return parseSexpr(bytes)

}

func parseSexpr(s []byte) (sexpr Sexpr, rest []byte, err error) {
	first, rest := s[0], s[1:]
	switch {
	case first == byte('('):
		return parseList(rest)
	case bytes.IndexByte(stringChar, first) > -1:
		return parseAtom(s)
	default:
		return nil, rest, fmt.Errorf("Unrecognised character at start of s-expression: %c", first)
	}

	panic("Should never get here")
}

func parseList(s []byte) (l List, rest []byte, err error) {
	acc := make(List, 0)
	var sexpr Sexpr
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == byte(')'):
			return acc, s[i:], nil
		case bytes.IndexByte(whitespaceChar, c) == -1:
			sexpr, s, err = parseSexpr(s[i:])
			if err != nil {
				return nil, nil, err
			}
			i = -1
			acc = append(acc, sexpr)
		}
	}
	return nil, nil, fmt.Errorf("Expected ')' to terminate list")
}

func parseAtom(s []byte) (a Atom, rest []byte, err error) {
	first, rest := s[0], s[1:]
	var displayHint, value []byte
	if first == byte('[') {
		displayHint, s, err = parseSimpleString(rest)
		if err != nil {
			return Atom{}, rest, err
		}
	}
	value, rest, err = parseSimpleString(s)
	if err != nil {
		return Atom{}, nil, err
	}
	return Atom{DisplayHint: displayHint, Value: value}, rest, nil
}

func parseSimpleString(s []byte) (str, rest []byte, err error) {
	length := -1
	if bytes.IndexByte(decimalDigit, s[0]) > -1 {
		var lengthString []byte
		lengthString, s, err = parseDecimal(s)
		if err != nil {
			return nil, nil, err
		}
		length, err = strconv.Atoi(string(lengthString))
		if err != nil {
			return nil, nil, err
		}
	}
	switch s[0] {
	case byte(':'):
		if length < 0 {
			return nil, nil, fmt.Errorf("Unspecified length of raw string")
		}
		return s[1 : length+1], s[length+1:], nil
	case byte('#'):
		str, rest, err = parseHexadecimal(s[1:])
	case byte('|'):
		str, rest, err = parseBase64(s[1:])
	case byte('"'):
		str, rest, err = parseQuotedString(s[1:], length)
	default:
		if bytes.IndexByte(tokenChar, s[0]) > -1 {
			var i int
			for i = 1; i < len(s) && bytes.IndexByte(tokenChar, s[i]) > -1; i++ {
			}
			str = s[0:i]
			return str, s[i:], nil
		}
		return nil, nil, fmt.Errorf("Unknown char %c parsing simple string")
	}
	if err != nil {
		return nil, nil, err
	}
	if length != -1 {
		if len(str) != length {
			return nil, nil, fmt.Errorf("Explicit length %d not equal to implicit length %d", length, len(str))
		}
		return str, s[length:], nil
	}
	return str, rest, nil
}

func parseDecimal(s []byte) (decimal, rest []byte, err error) {
	for i := range s {
		if bytes.IndexByte(decimalDigit, s[i]) < 0 {
			return s[0:i], s[i:], nil
		}
	}
	return s, rest, nil
}

func parseHexadecimal(s []byte) (str, rest []byte, err error) {
	for i := range s {
		if bytes.IndexByte(hexadecimalDigit, s[i]) < 0 {
			if s[i] != byte('#') {
				return nil, nil, fmt.Errorf("Expected # to terminate hexadecimal string; found %c", s[i])
			}
			str := make([]byte, hex.DecodedLen(i))
			length, err := hex.Decode(str, s[0:i])
			if err != nil {
				return nil, nil, err
			}
			return str[:length], s[i+1:], nil
		}
	}
	return nil, nil, fmt.Errorf("Unexpected end of hexadecimal value")
}

func parseBase64(s []byte) (decimal, rest []byte, err error) {
	for i := range s {
		if bytes.IndexByte(hexadecimalDigit, s[i]) < 0 {
			if s[i] != byte('|') {
				return nil, nil, fmt.Errorf("Expected | to terminate Base64 string")
			}
			base64 := s[0:i]
			decimal = make([]byte, base64Decoder.DecodedLen(len(base64)))
			length, err := base64Decoder.Decode(decimal, base64)
			if err != nil {
				return nil, nil, err
			}
			return base64[:length], s[i:], nil
		}
	}
	return nil, nil, fmt.Errorf("Unexpected end of Base64 value")
}

func parseQuotedString(s []byte, length int) (decimal, rest []byte, err error) {
	var acc []byte
	if length > 0 {
		acc = make([]byte, length)
	} else {
		acc = make([]byte, 0)
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case byte('"'):
			if length != -1 && len(acc) != length {
				return nil, nil, fmt.Errorf("Explicit length %d not equal to implicit length %d", length, len(acc))
			}
			return acc, s[i+1:], nil
		case '\\':
			i++
			if i == len(s) {
				return nil, nil, fmt.Errorf("Unterminated quoted string")
			}
			c = s[i]
			switch c {
			case byte('b'):
				c = byte('\b')
			case byte('t'):
				c = byte('\t')
			case byte('v'):
				c = byte('\v')
			case byte('n'):
				c = byte('\n')
			case byte('f'):
				c = byte('\f')
			case byte('r'):
				c = byte('\r')
			case byte('"'):
				c = byte('"')
			case byte('\''):
				c = byte('\'')
			case byte('\\'):
				c = byte('\\')
			case byte('\n'):
				if i+1 < len(s) && s[i+1] == byte('\r') {
					i++
				}
				continue
			case byte('\r'):
				if i+1 < len(s) && s[i+1] == byte('\n') {
					i++
				}
				continue
			case byte('x'):
				num, err := strconv.ParseInt(string(s[i+1:i+2]), 16, 8)
				if err != nil {
					return nil, nil, err
				}
				c = byte(num)
			default:
				if bytes.IndexByte(octalDigit, c) > -1 && bytes.IndexByte(octalDigit, s[i+1]) > -1 && bytes.IndexByte(octalDigit, s[i+2]) > -1 {
					num, err := strconv.ParseInt(string(s[i:i+2]), 8, 8)
					if err != nil {
						return nil, nil, err
					}
					c = byte(num)
				}
				return nil, nil, fmt.Errorf("Unrecognised escape character %c", rune(c))
			}
			fallthrough
		default:
			acc = append(acc, c)
		}
	}
	return nil, nil, fmt.Errorf("Unexpected end of quoted string")
}
