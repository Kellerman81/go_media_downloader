package logger

// LuceneEscapeTo writes the Lucene-escaped form of s into buf.
// Special characters: + - && || ! ( ) { } [ ] ^ " ~ * ? : \ /.
func LuceneEscapeTo(buf *AddBuffer, s string) {
	for _, c := range s {
		switch c {
		case '+', '-', '!', '(', ')', '{', '}', '[', ']', '^', '"', '~', '*', '?', ':', '\\', '/':
			buf.WriteByte('\\')
		case '&', '|':
			// && and || are two-character operators; escape each rune individually.
			buf.WriteByte('\\')
		}

		buf.WriteRune(c)
	}
}

// LuceneEscape escapes all Lucene special characters in s so the value is safe
// to embed inside a quoted phrase or unquoted term in a Lucene/Solr search
// query (MusicBrainz, OpenLibrary).
func LuceneEscape(s string) string {
	buf := PlAddBuffer.Get()
	defer PlAddBuffer.Put(buf)

	LuceneEscapeTo(buf, s)

	return buf.String()
}
