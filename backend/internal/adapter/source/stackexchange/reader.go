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
	table          domainingestion.Table
	reader         *bufio.Reader
	member         io.ReadCloser
	offset         int64
	maxRecordBytes int
}

type rawRow struct {
	XMLName    xml.Name
	Attributes []xml.Attr `xml:",any,attr"`
	InnerXML   string     `xml:",innerxml"`
}

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
			return domainingestion.SourceRecord{}, readErr
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
			return domainingestion.SourceRecord{}, io.EOF
		}

		trimLine := bytes.TrimSpace(rawLine)

		if len(trimLine) == 0 || isXMLDeclaration(trimLine) || bytes.Equal(trimLine, rootOpening) || bytes.Equal(trimLine, rootClosing) {

			if readErr == io.EOF {
				return domainingestion.SourceRecord{}, io.EOF
			}
			continue
		}
		if !bytes.HasPrefix(trimLine, []byte("<row")) {
			return domainingestion.SourceRecord{}, usecaseingestion.SourceError{
				Table:  s.table,
				Offset: lineOffset,
				Err:    domainingestion.ErrUnexpectedSourceLine,
			}
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
	return bytes.HasPrefix(line, []byte("<?xml ")) && bytes.HasSuffix(line, []byte("?>"))
}
func (s *recordStream) Close() error {
	return s.member.Close()
}
