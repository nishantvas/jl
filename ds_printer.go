package jl

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// DSCustomPrinter can print logs in a variety of compact formats, specified by FieldFormats.
type DSCustomPrinter struct {
	Out io.Writer
	// Disable colors disables adding color to fields.
	DisableColor bool
	// Disable truncate disables the Ellipsize and Truncate transforms.
	DisableTruncate bool
	// FieldFormats specifies the format the printer should use for logs. It defaults to DefaultCompactPrinterFieldFmt. Fields
	// are formatted in the order they are provided. If a DSFieldFmt produces a field that does not end with a whitespace,
	// a space character is automatically appended.
	FieldFormats []DSFieldFmt
}

// DSFieldFmt specifies a single field formatted by the DSCustomPrinter.
type DSFieldFmt struct {
	// Name of the field. This is used to find the field by key name if Finders is not set.
	Name string
	// List of FieldFinders to use to locate the field. Finders are executed in order until the first one that returns
	// non-nil.
	Finders []FieldFinder
	// Takes the output of the Finder and turns it into a string. If not set, DefaultStringer is used.
	Stringer Stringer
	// List of transformers to run on the field found to format the field.
	Transformers []Transformer
}

// DefaultCompactPrinterFieldFmt is a format for the DSCustomPrinter that tries to present logs in an easily skimmable manner
// for most types of logs.
var DefaultDSCustomPrinterFieldFmt = []DSFieldFmt{{
	Name:         "level",
	Finders:      []FieldFinder{ByNames("level", "severity")},
	Transformers: []Transformer{Truncate(4), UpperCase, ColorMap(LevelColors)},
}, {
	Name:    "ts",
	Finders: []FieldFinder{ByNames("timestamp", "time", "ts")},
}, {
	Name:         "thread",
	Transformers: []Transformer{Ellipsize(16), Format("[%s]"), RightPad(18), ColorSequence(AllColors)},
}, {
	Name:         "logger",
	Transformers: []Transformer{Ellipsize(30), Format("%s|"), LeftPad(21), ColorSequence(AllColors)},
}, {
	Name:    "msg",
	Finders: []FieldFinder{ByNames("message", "msg", "textPayload", "jsonPayload.message")},
}, {
	Name:     "stackTrace",
	Finders:  []FieldFinder{DSErrorFinder, ByNames("stackTrace", "stack")},
	Stringer: ErrorStringer,
}}

// NewCompactPrinter allocates and returns a new compact printer.
func NewDSCustomPrinter(w io.Writer) *DSCustomPrinter {
	return &DSCustomPrinter{
		Out:          w,
		FieldFormats: DefaultDSCustomPrinterFieldFmt,
	}
}

func (p *DSCustomPrinter) Print(entry *Entry) {
	if entry.Partials == nil {
		fmt.Fprintln(p.Out, string(entry.Raw))
		return
	}
	for i, fieldFmt := range p.FieldFormats {
		ctx := Context{
			DisableColor:    p.DisableColor,
			DisableTruncate: p.DisableTruncate,
		}
		formattedField := fieldFmt.format(&ctx, entry)
		if formattedField != "" {
			if i != 0 && !strings.HasPrefix(formattedField, "\n") {
				p.Out.Write([]byte(" "))
			}
			p.Out.Write([]byte(formattedField))
		}
	}
	p.Out.Write([]byte("\n"))
}

func (f *DSFieldFmt) format(ctx *Context, entry *Entry) string {
	var v interface{}
	// Find the value
	if len(f.Finders) > 0 {
		for _, finder := range f.Finders {
			if v = finder(entry); v != nil {
				break
			} else {
			}
		}
	} else {
		v = entry.Partials[f.Name]
	}
	if v == nil {
		return ""
	}

	// Stringify the value
	var s string
	if f.Stringer != nil {
		s = f.Stringer(ctx, v)
	} else {
		s = DefaultStringer(ctx, v)
	}
	s = strings.TrimRightFunc(s, unicode.IsSpace)

	if s == "" {
		return ""
	}

	original := s
	ctx.Original = original
	// Apply transforms
	for _, transform := range f.Transformers {
		s = transform.Transform(ctx, s)
	}

	if s == "" {
		return ""
	}

	return s
}
