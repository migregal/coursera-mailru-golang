package main

import (
	"fmt"
	"io"
	"bufio"
	"os"
	"strings"
	"strconv"
	json "encoding/json"
	easyjson "github.com/mailru/easyjson"
	jlexer "github.com/mailru/easyjson/jlexer"
	jwriter "github.com/mailru/easyjson/jwriter"
)

type User struct {
	Browsers []string
	Email    string
	Name     string
}

func FastSearch(out io.Writer) {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var count = 0
	var i = -1

	var users = make([]string, 0, 100)
	var checked = make(map[string]bool, 256)

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			break
		}

        var user = User{}
		err = user.UnmarshalJSON([]byte(line))
		if err != nil {
			panic(err)
		}

		var userAndroid = false
		var userMSIE = false

		for _, browser := range user.Browsers {
			isBrowserChecked, found := checked[browser]

			var MSIE = false
			var Android = false

			if !found {
				MSIE = strings.Contains(browser, "MSIE")
				Android = strings.Contains(browser, "Android")
				checked[browser] = MSIE || Android
			} else {
				if isBrowserChecked {
					MSIE = strings.Contains(browser, "MSIE")
					Android = strings.Contains(browser, "Android")
				} else {
					MSIE = false
					Android = false
				}
			}

			userMSIE = userMSIE || Android
			userAndroid = userAndroid || MSIE

			if !found && (Android || MSIE) {
				count++
			}
		}

		i++
		if !(userAndroid && userMSIE) {
			continue
		}

		var email = strings.Split(user.Email, "@")
		users = append(
			users,
			"[" + strconv.Itoa(i) + "] " +
			user.Name +
			" <" + email[0] + " [at] " + email[1] + ">",
		)

	}

	fmt.Fprintln(out, "found users:\n" + strings.Join(users, "\n") + "\n")
	fmt.Fprintln(out, "Total unique browsers", count)
}

// suppress unused package warning
var (
	_ *json.RawMessage
	_ *jlexer.Lexer
	_ *jwriter.Writer
	_ easyjson.Marshaler
)

func easyjson89aae3efDecodeFast(in *jlexer.Lexer, out *User) {
	isTopLevel := in.IsStart()
	if in.IsNull() {
		if isTopLevel {
			in.Consumed()
		}
		in.Skip()
		return
	}
	in.Delim('{')
	for !in.IsDelim('}') {
		key := in.UnsafeFieldName(false)
		in.WantColon()
		if in.IsNull() {
			in.Skip()
			in.WantComma()
			continue
		}
		switch key {
		case "name":
			out.Name = string(in.String())
		case "email":
			out.Email = string(in.String())
		case "browsers":
			if in.IsNull() {
				in.Skip()
				out.Browsers = nil
			} else {
				in.Delim('[')
				if out.Browsers == nil {
					if !in.IsDelim(']') {
						out.Browsers = make([]string, 0, 4)
					} else {
						out.Browsers = []string{}
					}
				} else {
					out.Browsers = (out.Browsers)[:0]
				}
				for !in.IsDelim(']') {
					var v1 string
					v1 = string(in.String())
					out.Browsers = append(out.Browsers, v1)
					in.WantComma()
				}
				in.Delim(']')
			}
		default:
			in.SkipRecursive()
		}
		in.WantComma()
	}
	in.Delim('}')
	if isTopLevel {
		in.Consumed()
	}
}
func easyjson89aae3efEncodeFast(out *jwriter.Writer, in User) {
	out.RawByte('{')
	first := true
	_ = first
	{
		const prefix string = ",\"name\":"
		out.RawString(prefix[1:])
		out.String(string(in.Name))
	}
	{
		const prefix string = ",\"email\":"
		out.RawString(prefix)
		out.String(string(in.Email))
	}
	{
		const prefix string = ",\"browsers\":"
		out.RawString(prefix)
		if in.Browsers == nil && (out.Flags&jwriter.NilSliceAsEmpty) == 0 {
			out.RawString("null")
		} else {
			out.RawByte('[')
			for v2, v3 := range in.Browsers {
				if v2 > 0 {
					out.RawByte(',')
				}
				out.String(string(v3))
			}
			out.RawByte(']')
		}
	}
	out.RawByte('}')
}

// MarshalJSON supports json.Marshaler interface
func (v User) MarshalJSON() ([]byte, error) {
	w := jwriter.Writer{}
	easyjson89aae3efEncodeFast(&w, v)
	return w.Buffer.BuildBytes(), w.Error
}

// MarshalEasyJSON supports easyjson.Marshaler interface
func (v User) MarshalEasyJSON(w *jwriter.Writer) {
	easyjson89aae3efEncodeFast(w, v)
}

// UnmarshalJSON supports json.Unmarshaler interface
func (v *User) UnmarshalJSON(data []byte) error {
	r := jlexer.Lexer{Data: data}
	easyjson89aae3efDecodeFast(&r, v)
	return r.Error()
}
