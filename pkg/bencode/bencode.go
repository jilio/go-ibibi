package bencode

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

// Strings are length-prefixed base ten followed by a colon and the string.
// For example 4:spam corresponds to 'spam'.

// Integers are represented by an 'i' followed by the number in base 10 followed by an 'e'.
// For example i3e corresponds to 3 and i-3e corresponds to -3. Integers have no size limitation.
// i-0e is invalid. All encodings with a leading zero, such as i03e, are invalid, other than i0e,
// which of course corresponds to 0.

// Lists are encoded as an 'l' followed by their elements (also bencoded) followed by an 'e'.
// For example l4:spam4:eggse corresponds to ['spam', 'eggs'].

// Dictionaries are encoded as a 'd' followed by a list of alternating keys and their
// corresponding values followed by an 'e'.
// For example, d3:cow3:moo4:spam4:eggse corresponds to {'cow': 'moo', 'spam': 'eggs'}
// and d4:spaml1:a1:bee corresponds to {'spam': ['a', 'b']}.
// Keys must be strings and appear in sorted order (sorted as raw strings, not alphanumerics).

func Decode(r *bufio.Reader) (interface{}, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch b {
	case 'i':
		numStr, err := r.ReadSlice('e')
		if err != nil {
			return nil, err
		}
		numStr = numStr[:len(numStr)-1]

		num, err := strconv.Atoi(string(numStr))
		if err != nil {
			return nil, err
		}
		return num, nil

	case 'l':
		list := []interface{}{}

		for {
			b, err := r.ReadByte()
			if err != nil {
				return nil, err
			}

			if b == 'e' {
				return list, nil
			} else {
				r.UnreadByte()
			}

			item, err := Decode(r)
			if err != nil {
				return nil, err
			}

			list = append(list, item)
		}

	case 'd':
		dict := map[string]interface{}{}

		for {
			b, err := r.ReadByte()
			if err != nil {
				return nil, err
			}

			if b == 'e' {
				return dict, nil
			} else {
				err = r.UnreadByte()
				if err != nil {
					return nil, err
				}
			}

			key, err := Decode(r)
			if err != nil {
				return nil, err
			}
			_, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("dict key is not a string")
			}

			value, err := Decode(r)
			if err != nil {
				return nil, err
			}

			dict[key.(string)] = value
		}

	default: // string
		err = r.UnreadByte()
		if err != nil {
			return nil, err
		}

		chunk, err := r.ReadSlice(':')
		if err != nil {
			return nil, err
		}
		chunk = chunk[:len(chunk)-1]

		strLen, err := strconv.Atoi(string(chunk))
		if err != nil {
			return nil, err
		}

		str := make([]byte, strLen)

		_, err = io.ReadFull(r, str)
		if err != nil {
			return nil, err
		}

		return string(str), nil
	}
}
