package diff

import (
	"sort"
	"strings"
	"testing"
)

// permutations returns every possible permutation of the given slice's elements.
// for example, for ["1", "2"], it returns [["1", "2"], ["2", "1"]]
func permutations(slice []string) [][]string {
	if len(slice) == 1 {
		return [][]string{slice}
	}
	var result [][]string
	for i, v := range slice {
		// copy slice to avoid mutating original
		sliceCopy := make([]string, len(slice))
		copy(sliceCopy, slice)
		// remove the current element from the slice
		sliceCopy = append(sliceCopy[:i], sliceCopy[i+1:]...)
		// get all permutations of the remaining elements
		permutations := permutations(sliceCopy)
		// append the current element to each permutation
		for _, permutation := range permutations {
			result = append(result, append(permutation, v))
		}
	}
	return result
}

func Test_xheadersLessFunc(t *testing.T) {
	possibleHeaders := []string{
		"old mode 100755",
		"new mode 100644",
		"diff --git a/file1 b/file2",
		"similarity index 70%",
		"rename from file1",
		"rename to file2",
		"index a9f9a6b..b0e5e4f",
	}
	tests := []struct{ input []string }{}
	for _, permutation := range permutations(possibleHeaders) {
		tests = append(tests, struct{ input []string }{input: permutation})
	}

	for _, tc := range tests {
		sort.Slice(tc.input, func(i, j int) bool {
			return xheadersLessFunc(tc.input[i], tc.input[j])
		})
		if !strings.HasPrefix(tc.input[0], "diff --git") {
			t.Errorf("xheadersLessFunc(%v): expected first element to be 'diff --git', got %v", tc.input, tc.input[0])
		}
		if !strings.HasPrefix(tc.input[1], "old mode") {
			t.Errorf("xheadersLessFunc(%v): expected second element to be 'old mode', got %v", tc.input, tc.input[1])
		}
		if !strings.HasPrefix(tc.input[2], "new mode") {
			t.Errorf("xheadersLessFunc(%v): expected third element to be 'new mode', got %v", tc.input, tc.input[2])
		}
	}
}

func TestReadQuotedFilename_Success(t *testing.T) {
	tests := []struct {
		input, value, remainder string
	}{
		{input: `""`, value: "", remainder: ""},
		{input: `"aaa"`, value: "aaa", remainder: ""},
		{input: `"aaa" bbb`, value: "aaa", remainder: " bbb"},
		{input: `"aaa" "bbb" ccc`, value: "aaa", remainder: ` "bbb" ccc`},
		{input: `"\""`, value: "\"", remainder: ""},
		{input: `"uh \"oh\""`, value: "uh \"oh\"", remainder: ""},
		{input: `"uh \\"oh\\""`, value: "uh \\", remainder: `oh\\""`},
		{input: `"uh \\\"oh\\\""`, value: "uh \\\"oh\\\"", remainder: ""},
	}
	for _, tc := range tests {
		value, remainder, err := readQuotedFilename(tc.input)
		if err != nil {
			t.Errorf("readQuotedFilename(`%s`): expected success, got '%s'", tc.input, err)
		} else if value != tc.value || remainder != tc.remainder {
			t.Errorf("readQuotedFilename(`%s`): expected `%s` and `%s`, got `%s` and `%s`", tc.input, tc.value, tc.remainder, value, remainder)
		}
	}
}

func TestReadQuotedFilename_Error(t *testing.T) {
	tests := []string{
		// Doesn't start with a quote
		``,
		`foo`,
		` "foo"`,
		// Missing end quote
		`"`,
		`"\"`,
		// "\x" is not a valid Go string literal escape
		`"\xxx"`,
	}
	for _, input := range tests {
		_, _, err := readQuotedFilename(input)
		if err == nil {
			t.Errorf("readQuotedFilename(`%s`): expected error", input)
		}
	}
}

func TestParseDiffGitArgs_Success(t *testing.T) {
	tests := []struct {
		input, first, second string
	}{
		{input: `aaa bbb`, first: "aaa", second: "bbb"},
		{input: `"aaa" bbb`, first: "aaa", second: "bbb"},
		{input: `aaa "bbb"`, first: "aaa", second: "bbb"},
		{input: `"aaa" "bbb"`, first: "aaa", second: "bbb"},
		{input: `1/a 2/z`, first: "1/a", second: "2/z"},
		{input: `1/hello world 2/hello world`, first: "1/hello world", second: "2/hello world"},
		{input: `"new\nline" and spaces`, first: "new\nline", second: "and spaces"},
		{input: `a/existing file with spaces "b/new, complicated\nfilen\303\270me"`, first: "a/existing file with spaces", second: "b/new, complicated\nfilen\303\270me"},
	}
	for _, tc := range tests {
		first, second, success := parseDiffGitArgs(tc.input)
		if !success {
			t.Errorf("`diff --git %s`: expected success", tc.input)
		} else if first != tc.first || second != tc.second {
			t.Errorf("`diff --git %s`: expected `%s` and `%s`, got `%s` and `%s`", tc.input, tc.first, tc.second, first, second)
		}
	}
}

func TestParseDiffGitArgs_Unsuccessful(t *testing.T) {
	tests := []string{
		``,
		`hello_world.txt`,
		`word `,
		` word`,
		`"a/bad_quoting b/bad_quoting`,
		`a/bad_quoting "b/bad_quoting`,
		`a/bad_quoting b/bad_quoting"`,
		`"a/bad_quoting b/bad_quoting"`,
		`"a/bad""b/bad"`,
		`"a/bad" "b/bad" "c/bad"`,
		`a/bad "b/bad" "c/bad"`,
	}
	for _, input := range tests {
		first, second, success := parseDiffGitArgs(input)
		if success {
			t.Errorf("`diff --git %s`: expected unsuccessful; got `%s` and `%s`", input, first, second)
		}
	}
}
