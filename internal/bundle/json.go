package bundle

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type jsonField struct {
	typ      reflect.Type
	required bool
}

var jsonFields sync.Map

func fieldsFor(typ reflect.Type) map[string]jsonField {
	if cached, ok := jsonFields.Load(typ); ok {
		return cached.(map[string]jsonField)
	}
	fields := map[string]jsonField{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")
		if tag[0] == "" || tag[0] == "-" {
			continue
		}
		fields[tag[0]] = jsonField{typ: field.Type, required: !allowed("omitempty", tag[1:]...)}
	}
	jsonFields.Store(typ, fields)
	return fields
}

type jsonScanner struct {
	ctx     context.Context
	decoder *json.Decoder
	tokens  int
}

func (s *jsonScanner) token() (json.Token, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	s.tokens++
	if s.tokens > 200_000 {
		return nil, ErrBounds
	}
	t, err := s.decoder.Token()
	if err != nil {
		return nil, ErrInvalid
	}
	if value, ok := t.(string); ok && len(value) > 4096 {
		return nil, ErrBounds
	}
	if value, ok := t.(json.Number); ok && len(value) > 32 {
		return nil, ErrBounds
	}
	return t, nil
}

func (s *jsonScanner) value(typ reflect.Type, depth int) error {
	if depth > 32 {
		return ErrBounds
	}
	t, err := s.token()
	if err != nil {
		return err
	}
	if typ.Kind() == reflect.Pointer {
		// Only the required legacy identity presence markers are nullable.
		if t == nil && typ.Elem().Kind() == reflect.String {
			return nil
		}
		typ = typ.Elem()
	}
	if typ == reflect.TypeFor[time.Time]() {
		value, ok := t.(string)
		if !ok || !strings.HasSuffix(value, "Z") {
			return ErrInvalid
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || parsed.IsZero() || parsed.Format(time.RFC3339Nano) != value {
			return ErrInvalid
		}
		return nil
	}
	switch typ.Kind() {
	case reflect.Struct:
		if t != json.Delim('{') {
			return ErrInvalid
		}
		fields, seen := fieldsFor(typ), map[string]bool{}
		for s.decoder.More() {
			key, err := s.token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return ErrInvalid
			}
			field, ok := fields[name]
			if !ok {
				return ErrInvalid
			}
			seen[name] = true
			if err := s.value(field.typ, depth+1); err != nil {
				return err
			}
		}
		end, err := s.token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return ErrInvalid
		}
		for name, field := range fields {
			if field.required && !seen[name] {
				return ErrInvalid
			}
		}
	case reflect.Slice:
		if t != json.Delim('[') {
			return ErrInvalid
		}
		count := 0
		for s.decoder.More() {
			count++
			if count > 4096 {
				return ErrBounds
			}
			if err := s.value(typ.Elem(), depth+1); err != nil {
				return err
			}
		}
		end, err := s.token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return ErrInvalid
		}
	case reflect.String:
		if _, ok := t.(string); !ok {
			return ErrInvalid
		}
	case reflect.Bool:
		if _, ok := t.(bool); !ok {
			return ErrInvalid
		}
	case reflect.Int, reflect.Int64:
		number, ok := t.(json.Number)
		if !ok {
			return ErrInvalid
		}
		if _, err := strconv.ParseInt(string(number), 10, typ.Bits()); err != nil {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

// Preflight grammar/work/required fields without constructing an interface{}
// graph. Exact JSON tags also reject encoding/json's case-insensitive aliases.
func decodeJSON(ctx context.Context, data []byte, destination any) error {
	if !utf8.Valid(data) {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	s := jsonScanner{ctx: ctx, decoder: decoder}
	if err := s.value(reflect.TypeOf(destination).Elem(), 1); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return ErrInvalid
	}
	return nil
}
