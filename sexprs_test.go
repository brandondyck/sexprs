// Copyright 2013 Robert A. Uhl.  All rights reserved.
// Use of this source code is goverend by an MIT-style license which may
// be found in the LICENSE file.

package sexprs_test

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/eadmund/sexprs"
	"pgregory.net/rapid"
)

func TODO(t *testing.T) {
	t.Fatalf("TODO")
}

func Atom() *rapid.Generator[sexprs.Atom] {
	return rapid.Custom(func(t *rapid.T) sexprs.Atom {
		return sexprs.Atom{
			DisplayHint: rapid.SliceOf(rapid.Byte()).Draw(t, "DisplayHint"),
			Value:       rapid.SliceOf(rapid.Byte()).Draw(t, "Value"),
		}
	})
}

func List() *rapid.Generator[sexprs.List] {
	return rapid.Custom(func(t *rapid.T) sexprs.List {
		var s []sexprs.Sexp = rapid.SliceOfN(rapid.Deferred(Sexp), 0, 5).Draw(t, "s")
		return sexprs.List(s)
	})
}

func AsSexp[T sexprs.Sexp](value T) sexprs.Sexp {
	return sexprs.Sexp(value)
}

func Sexp() *rapid.Generator[sexprs.Sexp] {
	return rapid.Custom(func(t *rapid.T) sexprs.Sexp {
		choice := rapid.Uint8Range(0, 10).Draw(t, "choice")
		if choice < 7 {
			return rapid.Map(Atom(), AsSexp).Draw(t, "atom-sexp")
		}
		return rapid.Map(rapid.Deferred(List), AsSexp).Draw(t, "list-sexp")
	})
}

func TestPackAndParseEqual(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sexp := Sexp().Draw(t, "sexp")
		packed := sexp.Pack()
		parsed, rest, err := sexprs.Parse(packed)
		if err != nil {
			t.Fatalf("failed to parse sexp: %v", err)
		}
		if len(rest) != 0 {
			t.Errorf("unexpected remaining bytes after parsing sexp: %v", rest)
		}
		if parsed == nil {
			t.Fatal("parsed sexp is nil")
		}
		if !parsed.Equal(sexp) || !sexp.Equal(parsed) {
			t.Fatalf("result not equal to original sexp\nexpected %q\ngot %q", packed, parsed.Pack())
		}
	})
}

func TestTransportAndParseEqual(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		sexp := Sexp().Draw(t, "sexp")
		transport := sexp.Base64String()
		parsed, rest, err := sexprs.Parse([]byte(transport))
		if err != nil {
			t.Fatalf("failed to parse sexp: %v", err)
		}
		if len(rest) != 0 {
			t.Errorf("unexpected remaining bytes after parsing sexp: %v", rest)
		}
		if parsed == nil {
			t.Fatal("parsed sexp is nil")
		}
		if !parsed.Equal(sexp) || !sexp.Equal(parsed) {
			t.Fatalf("result not equal to original sexp\nexpected %q\ngot %q", transport, parsed.Pack())
		}
	})
}

func MustBeEqual(t rapid.TB, s1, s2 sexprs.Sexp) {
	e1 := s1.Equal(s2)
	e2 := s2.Equal(s1)
	if e1 == !e2 {
		t.Logf("expected Sexp.Equal to be commutative, but got different results")
	}
	if !(e1 && e2) {
		t.Fatalf("expected sexps to be equal\ns1: %q\ns2: %q", string(s1.Pack()), string(s2.Pack()))
	}
}

func MustNotBeEqual(t rapid.TB, s1, s2 sexprs.Sexp) {
	e1 := s1.Equal(s2)
	e2 := s2.Equal(s1)
	if e1 == !e2 {
		t.Logf("expected Sexp.Equal to be commutative, but got different results")
	}
	if e1 || e2 {
		t.Fatalf("expected sexps not to be equal\ns1: %q\ns2: %q", string(s1.Pack()), string(s2.Pack()))
	}
}

func TestNotEqual(t *testing.T) {
	t.Run("add item to list", TODO)
	t.Run("wrap in list", TODO)
	t.Run("remove item from list", TODO)
	t.Run("change atom's hint", TODO)
	t.Run("change atom's value", TODO)
}

func TestAtomPack(t *testing.T) {
	a := sexprs.Atom{Value: []byte("This is a test")}
	b := a.Pack()
	if !bytes.Equal(b, []byte("14:This is a test")) {
		t.Fail()
	}
}

func assertPacked(t *testing.T, sexp sexprs.Sexp, expected string) {
	t.Helper()
	packed := sexp.Pack()
	if !bytes.Equal([]byte(expected), packed) {
		t.Fatalf("unexpected canonical representation\nexpected %q\ngot %q", expected, string(packed))
	}
}

func TestList(t *testing.T) {
	a := sexprs.Atom{Value: []byte("This is a test")}
	l := sexprs.List{a}
	assertPacked(t, l, "(14:This is a test)")
}

func TestError(t *testing.T) {
	s := "((a)"
	_, _, err := sexprs.Parse([]byte(s))
	if err == nil {
		t.Fatalf("Parsing %v should have produced an error", s)
	}
}

func testParse(t *testing.T, input, expectedOutput string) {
	t.Helper()
	t.Run(input, func(t *testing.T) {
		t.Helper()
		sexp, rest, err := sexprs.Parse([]byte(input))
		if err != nil {
			t.Fatalf("could not parse %q: %v", input, err)
		}
		if sexp == nil {
			t.Fatalf("nil result from parsing %q", input)
		}
		if len(rest) != 0 {
			t.Errorf("unexpected remaining input after parsing %q: %q", input, string(rest))
		}
		assertPacked(t, sexp, expectedOutput)
	})

}

func TestParse(t *testing.T) {
	testParse(t, "()", "()")
	testParse(t, "([text]test)", "([4:text]4:test)")
	testParse(t, "(4:test3:foo(baz))", "(4:test3:foo(3:baz))")
	testParse(t, "testing", "7:testing")
	testParse(t, `"testing-foo bar"`, "15:testing-foo bar")
	testParse(t, `("testing-foo bar")`, "(15:testing-foo bar)")
	testParse(t, `(testing-foo" bar")`, "(11:testing-foo4: bar)")
	testParse(t,
		`([foo/bar]#7a # ["quux beam"]bar ([jim]|Zm9vYmFy YmF6|)"foo bar\r"{Zm9vYmFyYmF6})`,
		"([7:foo/bar]1:z[9:quux beam]3:bar([3:jim]9:foobarbaz)8:foo bar\r9:foobarbaz)",
	)
}

func TestIsList(t *testing.T) {
	TODO(t) // convert to prop test
	s, _, err := sexprs.Parse([]byte("(abc efg-hijk )"))
	if err != nil {
		t.Fatal("Could not parse list", err)
	}
	if !sexprs.IsList(s) {
		t.Fatal("List considered not-List")
	}
	s, _, err = sexprs.Parse([]byte("abc"))
	if err != nil {
		t.Fatal("Could not parse atom", err)
	}
	if sexprs.IsList(s) {
		t.Fatal("Atom considered List")
	}
}

func TestRead(t *testing.T) {
	TODO(t) // consider converting to prop test
	s, err := sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("()"))))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := s.(sexprs.List)
	if !ok {
		t.Fatal("List expected")
	}
	if len(l) != 0 {
		t.Fatal("Zero-length list expected")
	}
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("6:foobar"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok := s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	t.Log(string(a.Value), len(a.Value))
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("7:foobar"))))
	if err == nil {
		t.Fatal("Didn't fail on invalid bytestring")
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("foobar")) {
		t.Fatal("bad ", a)
	}
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("3#61 6 263#"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("abc")) {
		t.Fatal("Bad ", a)
	}
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("3|Y2\r\nJ h|"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("cba")) {
		t.Fatal("Bad ", a)
	}
	//t.Log(">>", string(a.Value))
	// hex without length
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("#616263#"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("abc")) {
		t.Fatal("Bad ", a)
	}
	// base64 without length
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("|Y2Jh|"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("cba")) {
		t.Fatal("Bad ", a)
	}
	// quoted string without length
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("\"Foo bar \rbaz quux\\\nquuux\""))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("Foo bar \rbaz quuxquuux")) {
		t.Fatal("Bad ", a)
	}
	// escaped return
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("\"Foo bar \\\r\""))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	a, ok = s.(sexprs.Atom)
	if !ok {
		t.Fatal("Atom expected")
	}
	if !bytes.Equal(a.Value, []byte("Foo bar ")) {
		t.Fatal("Bad ", a)
	}
	// list
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("(a b)"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	l, ok = s.(sexprs.List)
	if !ok {
		t.Fatal("List expected")
	}
	if !l.Equal(sexprs.List{sexprs.Atom{Value: []byte("a")}, sexprs.Atom{Value: []byte("b")}}) {
		t.Fatal("Bad ", l)
	}
	// display hint
	s, err = sexprs.Read(bufio.NewReader(bytes.NewReader([]byte("[abc]bar"))))
	if err != nil && err != io.EOF {
		t.Fatal(err)
	}
	if !s.Equal(sexprs.Atom{DisplayHint: []byte("abc"), Value: []byte("bar")}) {
		t.Fatal("Bad s-expression", s)
	}
}

func ExampleAtom_Pack() {
	foo := sexprs.Atom{Value: []byte("foo")}
	fmt.Println(string(foo.Pack()))
	bar := sexprs.Atom{DisplayHint: []byte("text/plain"), Value: []byte("bar")}
	fmt.Println(string(bar.Pack()))
	// Output:
	// 3:foo
	// [10:text/plain]3:bar
}

func ExampleAtom_String() {
	foo := sexprs.Atom{Value: []byte("foo")}
	fmt.Println(foo.String())
	foo.Value = []byte("bar baz")
	fmt.Println(foo.String())
	foo.Value = []byte("bar\nbaz")
	fmt.Println(foo.String())
	foo.Value = []byte{0, 1, 2, 3}
	fmt.Println(foo.String())
	// Output:
	// foo
	// "bar baz"
	// "bar\nbaz"
	// |AAECAw==|
}

func ExampleList_Pack() {
	list := sexprs.List{sexprs.Atom{Value: []byte("foo")}, sexprs.List{sexprs.Atom{Value: []byte("bar baz"), DisplayHint: []byte("text/plain")}, sexprs.Atom{DisplayHint: []byte{'\n'}}}}
	fmt.Println(string(list.Pack()))
	readList, _, err := sexprs.Parse([]byte(list.String()))
	if err != nil {
		panic(err)
	}
	fmt.Println(readList.Equal(list))
	// Output:
	// (3:foo([10:text/plain]7:bar baz[1:
	// ]0:))
	// true
}

func ExampleList_String() {
	list := sexprs.List{sexprs.Atom{Value: []byte("foo")}, sexprs.List{sexprs.Atom{Value: []byte("bar baz"), DisplayHint: []byte("text/plain")}, sexprs.Atom{DisplayHint: []byte{1, 2, 3}}}}
	fmt.Println(list.String())
	readList, _, err := sexprs.Parse([]byte(list.String()))
	if err != nil {
		panic(err)
	}
	fmt.Println(readList.Equal(list))
	// Output:
	// (foo ([text/plain]"bar baz" [|AQID|]""))
	// true
}
