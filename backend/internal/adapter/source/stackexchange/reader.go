package stackexchange

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	domainingestion "quorum/internal/domain/ingestion"
	usecaseingestion "quorum/internal/usecase/ingestion"
	"strings"
)

type recordStream struct {
	table           domainingestion.Table
	reader          *bufio.Reader
	member          io.ReadCloser
	offset          int64
	maxRecordBytes  int
	declarationSeen bool
	rootOpened      bool
	rootClosed      bool
}

type rawRow struct {
	XMLName    xml.Name
	Attributes []xml.Attr `xml:",any,attr"`
	InnerXML   string     `xml:",innerxml"`
}

var errInvalidXMLDeclaration = errors.New("invalid XML declaration")

func newRecordStream(table domainingestion.Table, member io.ReadCloser, maxRecordBytes int) *recordStream {
	return &recordStream{
		table:          table,
		reader:         bufio.NewReaderSize(member, maxRecordBytes+2),
		member:         member,
		offset:         0,
		maxRecordBytes: maxRecordBytes,
	}
}
func (s *recordStream) Next(ctx context.Context) (domainingestion.SourceRecord, error) {
	rootName := tableToRoot[s.table]
	rootOpening := []byte("<" + rootName + ">")
	rootClosing := []byte("</" + rootName + ">")

	for {
		if err := ctx.Err(); err != nil {
			return domainingestion.SourceRecord{}, err
		}

		lineOffset := s.offset
		lineBytes, readErr := s.reader.ReadSlice('\n')

		s.offset += int64(len(lineBytes))

		if errors.Is(readErr, bufio.ErrBufferFull) {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    domainingestion.ErrRecordTooLarge,
			}
		}
		if readErr != nil && readErr != io.EOF {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    readErr,
			}
		}

		rawLen := len(lineBytes)
		if rawLen > 0 && lineBytes[rawLen-1] == '\n' {
			rawLen--
			if rawLen > 0 && lineBytes[rawLen-1] == '\r' {
				rawLen--
			}
		}
		rawLine := lineBytes[:rawLen]

		if len(rawLine) > s.maxRecordBytes {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    domainingestion.ErrRecordTooLarge,
			}
		}

		if readErr == io.EOF && len(lineBytes) == 0 {
			if s.rootClosed {
				return domainingestion.SourceRecord{}, io.EOF
			}
			return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
		}

		trimLine := bytes.TrimSpace(rawLine)
		if lineOffset == 0 {
			trimLine = bytes.TrimPrefix(trimLine, []byte{0xef, 0xbb, 0xbf})
		}

		if len(trimLine) == 0 {
			if readErr == io.EOF {
				if s.rootClosed {
					return domainingestion.SourceRecord{}, io.EOF
				}
				return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
			}
			continue
		}

		if isXMLDeclaration(trimLine) {
			if s.declarationSeen || s.rootOpened || s.rootClosed {
				return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
			}
			if err := validateXMLDeclaration(trimLine); err != nil {
				return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
					Table:  s.table,
					Offset: lineOffset,
					Err:    fmt.Errorf("%w: %v", domainingestion.ErrMalformedRecord, err),
				}
			}
			s.declarationSeen = true
			if readErr == io.EOF {
				return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
			}
			continue
		}

		if bytes.Equal(trimLine, rootOpening) {
			if s.rootOpened || s.rootClosed {
				return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
			}
			s.rootOpened = true
			if readErr == io.EOF {
				return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
			}
			continue
		}

		if bytes.Equal(trimLine, rootClosing) {
			if !s.rootOpened || s.rootClosed {
				return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
			}
			s.rootClosed = true
			if readErr == io.EOF {
				return domainingestion.SourceRecord{}, io.EOF
			}
			continue
		}

		if !s.rootOpened || s.rootClosed {
			return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
		}
		if !bytes.HasPrefix(trimLine, []byte("<row")) {
			return domainingestion.SourceRecord{}, s.unexpected(lineOffset)
		}

		var row rawRow
		if unmarshalErr := xml.Unmarshal([]byte(trimLine), &row); unmarshalErr != nil {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    fmt.Errorf("%w: %v", domainingestion.ErrMalformedRecord, unmarshalErr),
			}
		}

		if row.XMLName.Local != "row" {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    domainingestion.ErrUnexpectedSourceLine,
			}
		}

		if strings.TrimSpace(row.InnerXML) != "" {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    fmt.Errorf("%w: unexpected child elements or text found", domainingestion.ErrMalformedRecord),
			}
		}

		attrs := make(map[string]string, len(row.Attributes))
		for _, attr := range row.Attributes {
			if _, exists := attrs[attr.Name.Local]; exists {
				return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
					Table:  s.table,
					Offset: lineOffset,
					Err:    fmt.Errorf("%w: duplicate attribute %q", domainingestion.ErrMalformedRecord, attr.Name.Local),
				}
			}
			attrs[attr.Name.Local] = attr.Value
		}

		return domainingestion.NewSourceRecord(s.table, lineOffset, string(rawLine), attrs), nil
	}
}

func isXMLDeclaration(line []byte) bool {
	return bytes.HasPrefix(line, []byte("<?xml"))
}

func validateXMLDeclaration(line []byte) error {
	content := bytes.TrimSuffix(bytes.TrimPrefix(line, []byte("<?xml")), []byte("?>"))
	candidate := make([]byte, 0, len(content)+15)
	candidate = append(candidate, "<declaration"...)
	candidate = append(candidate, content...)
	candidate = append(candidate, "/>"...)

	var declaration rawRow
	if err := xml.Unmarshal(candidate, &declaration); err != nil {
		return err
	}

	versionSeen := false
	position := 0
	seen := make(map[string]struct{}, len(declaration.Attributes))
	for _, attribute := range declaration.Attributes {
		if attribute.Name.Space != "" {
			return errInvalidXMLDeclaration
		}
		if _, ok := seen[attribute.Name.Local]; ok {
			return errInvalidXMLDeclaration
		}
		seen[attribute.Name.Local] = struct{}{}
		switch attribute.Name.Local {
		case "version":
			if position != 0 || attribute.Value != "1.0" {
				return errInvalidXMLDeclaration
			}
			versionSeen = true
			position = 1
		case "encoding":
			if position != 1 || !validEncodingName(attribute.Value) {
				return errInvalidXMLDeclaration
			}
			position = 2
		case "standalone":
			if position < 1 || position > 2 || attribute.Value != "yes" && attribute.Value != "no" {
				return errInvalidXMLDeclaration
			}
			position = 3
		default:
			return errInvalidXMLDeclaration
		}
	}

	if !versionSeen {
		return errInvalidXMLDeclaration
	}
	return nil
}

func validEncodingName(value string) bool {
	if value == "" || !asciiLetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if asciiLetter(character) || character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func asciiLetter(character byte) bool {
	return character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

func (s *recordStream) unexpected(offset int64) usecaseingestion.SourceError {
	return usecaseingestion.SourceError{
		Table:  s.table,
		Offset: offset,
		Err:    domainingestion.ErrUnexpectedSourceLine,
	}
}

func (s *recordStream) Close() error {
	return s.member.Close()
}
