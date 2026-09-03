package message

import (
	"strings"
	"unicode/utf8"
)

const MaxMessageLength = 280

type Message struct {
	value string
}

func NewMessage(raw string) (Message, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Message{}, ErrEmpty
	}

	if utf8.RuneCountInString(trimmed) > MaxMessageLength {
		return Message{}, ErrTooLong
	}

	return Message{value: trimmed}, nil
}

func (m Message) String() string {
	return m.value
}

func (m Message) Length() int {
	return utf8.RuneCountInString(m.value)
}
