package utils

import "fmt"

// Transform a normal writer into a writer that
// splits the string into lines of 75 characters. Example:
//
//	var sb strings.Builder
//	writer := split75wrapper(sb.WriteString)
//	writer("Hello,world!")
//	fmt.Println(sb.String())
//
// Output: (let's assume it splits into 6-character lines)
//
//	`Hello,
//	 world!`
func Split75wrapper(writer func(string) (int, error)) func(string) (int, error) {
	return func(str string) (int, error) {
		// write right away if the string is short enough
		if len(str) <= 75 {
			if i, err := writer(str); err != nil {
				return i, err
			}
			return len(str), nil
		}

		// split every 75 characters
		// var slice []string
		slice := func() []string {
			var slice []string
			for i := 0; i < len(str); i += 75 {
				begin := i
				end := i + 75
				if end > len(str) {
					end = len(str)
				}
				slice = append(slice, str[begin:end])
			}
			return slice

		}()
		for i, s := range slice {
			switch i {
			case 0:
				if i, err := writer(fmt.Sprintf("%s\n", s)); err != nil {
					return i, err
				}
			case len(slice) - 1:
				if i, err := writer(s); err != nil {
					return i, err
				}
			default:
				if i, err := writer(fmt.Sprintf(" %s\n", s)); err != nil {
					return i, err
				}
			}
		}

		return len(str), nil
	}
}
