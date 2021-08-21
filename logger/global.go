package logger

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

func stringReplaceArray(instr string, what []string, with string) string {
	for _, line := range what {
		instr = strings.Replace(instr, line, with, -1)
	}
	return instr
}

//no chinese or cyrilic supported
func StringToSlug(instr string) string {
	instr = strings.ToLower(instr)
	instr = strings.Replace(instr, "ä", "ae", -1)
	instr = strings.Replace(instr, "ö", "oe", -1)
	instr = strings.Replace(instr, "ü", "ue", -1)
	instr = strings.Replace(instr, "ß", "ss", -1)
	instr = strings.Replace(instr, "&", "and", -1)
	instr = strings.Replace(instr, "@", "at", -1)
	instr = strings.Replace(instr, "½", ",5", -1)
	instr = strings.Replace(instr, "'", "", -1)
	instr = stringReplaceArray(instr, []string{" ", "§", "$", "%", "/", "(", ")", "=", "!", "?", "`", "\\", "}", "]", "[", "{", "|", ",", ".", ";", ":", "_", "+", "#", "<", ">", "*"}, "-")
	instr = strings.Replace(instr, "--", "-", -1)
	instr = strings.Replace(instr, "--", "-", -1)
	instr = strings.Replace(instr, "--", "-", -1)
	instr = strings.Replace(instr+"-", "-i-", "-1-", -1)
	instr = strings.Replace(instr, "-ii-", "-2-", -1)
	instr = strings.Replace(instr, "-iii-", "-3-", -1)
	instr = strings.Replace(instr, "-iv-", "-4-", -1)
	instr = strings.Replace(instr, "-v-", "-5-", -1)
	instr = strings.Replace(instr, "-vi-", "-6-", -1)
	instr = strings.Replace(instr, "-vii-", "-7-", -1)
	instr = strings.Replace(instr, "-viii-", "-8-", -1)
	instr = strings.Replace(instr, "-ix-", "-9-", -1)
	instr = strings.Replace(instr, "-x-", "-10-", -1)
	instr = strings.Replace(instr, "-xi-", "-11-", -1)
	instr = strings.Replace(instr, "-xii-", "-12-", -1)
	instr = strings.Replace(instr, "-xiii-", "-13-", -1)
	instr = strings.Replace(instr, "-xiv-", "-14-", -1)
	instr = strings.Replace(instr, "-xv-", "-15-", -1)
	instr = strings.Replace(instr, "-xvi-", "-16-", -1)
	instr = strings.Replace(instr, "-xvii-", "-17-", -1)
	instr = strings.Replace(instr, "-xviii-", "-18-", -1)
	instr = strings.Replace(instr, "-xix-", "-19-", -1)
	instr = strings.Replace(instr, "-xx-", "-20-", -1)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, instr)
	result = strings.Trim(result, "-")
	return result
}
