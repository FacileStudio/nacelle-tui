package theme

import (
	"encoding/json"
	"reflect"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

// colourless returns glamour's standard layout for the style name with every
// colour field cleared. A copy is taken first so repeated calls, and the
// package-level style configs themselves, are untouched.
func colourless(style string) ansi.StyleConfig {
	base := styles.LightStyleConfig
	if style == "dark" {
		base = styles.DarkStyleConfig
	}

	blob, err := json.Marshal(base)
	if err != nil {
		return styles.NoTTYStyleConfig
	}
	var out ansi.StyleConfig
	if err := json.Unmarshal(blob, &out); err != nil {
		return styles.NoTTYStyleConfig
	}
	strip(reflect.ValueOf(&out))
	return out
}

// isColourField names the string or pointer-to-string fields that carry a
// paint colour: glamour primitives (Color, BackgroundColor), chroma token
// styles (Colour, Background).
func isColourField(name string) bool {
	return name == "Color" || name == "BackgroundColor" ||
		name == "Colour" || name == "Background"
}

// strip recursively clears the colour fields of a style config: every string
// field the JSON names colour, background_color or similar, wherever it sits
// in the config — primitives, chroma token styles, or deeper. Structural
// fields (bold, indent, prefix) survive, which is what keeps the markdown
// legible without a palette.
func strip(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			strip(v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			stripField(v, i)
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			strip(v.MapIndex(key))
		}
	}
}

func stripField(v reflect.Value, i int) {
	field := v.Field(i)
	if isColourField(v.Type().Field(i).Name) {
		switch {
		case field.Kind() == reflect.String && field.CanSet():
			field.Set(reflect.Zero(field.Type()))
		case field.Kind() == reflect.Pointer && field.CanSet():
			field.Set(reflect.Zero(field.Type()))
		}
	}
	strip(field)
}
