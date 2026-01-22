package typeconv_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/spacecafe/go-parts/pkg/typeconv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConverter_Convert_String(t *testing.T) {
	t.Parallel()

	var result string

	target := reflect.ValueOf(&result).Elem()

	c := typeconv.New()
	err := c.Convert(target, "hello world")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestConverter_Convert_Int(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		target  any
		check   func(t *testing.T, target any)
		wantErr bool
	}{
		{
			name:   "int8",
			value:  "127",
			target: new(int8),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*int8)
				require.True(t, ok)
				assert.Equal(t, int8(127), *v)
			},
		},
		{
			name:    "int8 overflow",
			value:   "128",
			target:  new(int8),
			wantErr: true,
		},
		{
			name:   "int16",
			value:  "32767",
			target: new(int16),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*int16)
				require.True(t, ok)
				assert.Equal(t, int16(32767), *v)
			},
		},
		{
			name:   "int32",
			value:  "2147483647",
			target: new(int32),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*int32)
				require.True(t, ok)
				assert.Equal(t, int32(2147483647), *v)
			},
		},
		{
			name:   "int64",
			value:  "9223372036854775807",
			target: new(int64),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*int64)
				require.True(t, ok)
				assert.Equal(t, int64(9223372036854775807), *v)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := reflect.ValueOf(tt.target).Elem()
			c := typeconv.New()
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.check(t, tt.target)
			}
		})
	}
}

func TestConverter_Convert_Uint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		target  any
		check   func(t *testing.T, target any)
		wantErr bool
	}{
		{
			name:   "uint",
			value:  "42",
			target: new(uint),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*uint)
				require.True(t, ok)
				assert.Equal(t, uint(42), *v)
			},
		},
		{
			name:    "negative value",
			value:   "-1",
			target:  new(uint),
			wantErr: true,
		},
		{
			name:   "uint8",
			value:  "255",
			target: new(uint8),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*uint8)
				require.True(t, ok)
				assert.Equal(t, uint8(255), *v)
			},
		},
		{
			name:    "uint8 overflow",
			value:   "256",
			target:  new(uint8),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := reflect.ValueOf(tt.target).Elem()
			c := typeconv.New()
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.check(t, tt.target)
			}
		})
	}
}

func TestConverter_Convert_Float(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		target  any
		check   func(t *testing.T, target any)
		wantErr bool
	}{
		{
			name:   "float32",
			value:  "3.14",
			target: new(float32),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*float32)
				require.True(t, ok)
				assert.InDelta(t, float32(3.14), *v, 0.01)
			},
		},
		{
			name:   "float64",
			value:  "3.141592653589793",
			target: new(float64),
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*float64)
				require.True(t, ok)
				assert.InDelta(t, 3.141592653589793, *v, 0.000001)
			},
		},
		{
			name:    "invalid float",
			value:   "abc",
			target:  new(float64),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := reflect.ValueOf(tt.target).Elem()
			c := typeconv.New()
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.check(t, tt.target)
			}
		})
	}
}

func TestConverter_Convert_Bool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{"true", "true", true, false},
		{"t", "t", true, false},
		{"yes", "yes", true, false},
		{"y", "y", true, false},
		{"on", "on", true, false},
		{"1", "1", true, false},
		{"false", "false", false, false},
		{"f", "f", false, false},
		{"no", "no", false, false},
		{"n", "n", false, false},
		{"off", "off", false, false},
		{"0", "0", false, false},
		{"uppercase TRUE", "TRUE", true, false},
		{"uppercase FALSE", "FALSE", false, false},
		{"with spaces", " yes ", true, false},
		{"invalid", "invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var result bool

			target := reflect.ValueOf(&result).Elem()

			c := typeconv.New()
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, typeconv.ErrInvalidValue)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestConverter_Convert_Duration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{"seconds", "5s", 5 * time.Second, false},
		{"minutes", "10m", 10 * time.Minute, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"combined", "1h30m", 90 * time.Minute, false},
		{"milliseconds", "500ms", 500 * time.Millisecond, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var result time.Duration

			target := reflect.ValueOf(&result).Elem()

			c := typeconv.New()
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, typeconv.ErrInvalidValue)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestConverter_Convert_Time(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		value      string
		timeLayout string
		wantErr    bool
	}{
		{
			name:       "RFC3339",
			value:      "2023-01-15T10:30:00Z",
			timeLayout: time.RFC3339,
			wantErr:    false,
		},
		{
			name:       "custom layout",
			value:      "2023-01-15",
			timeLayout: "2006-01-02",
			wantErr:    false,
		},
		{
			name:       "invalid time",
			value:      "invalid",
			timeLayout: time.RFC3339,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var result time.Time

			target := reflect.ValueOf(&result).Elem()

			c := &typeconv.Converter{
				SliceSeparator: ",",
				TimeLayout:     tt.timeLayout,
			}
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				expected, _ := time.Parse(tt.timeLayout, tt.value)
				assert.Equal(t, expected, result)
			}
		})
	}
}

func TestConverter_Convert_Pointer(t *testing.T) {
	t.Parallel()

	var result *int

	target := reflect.ValueOf(&result).Elem()

	c := typeconv.New()
	err := c.Convert(target, "42")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 42, *result)
}

func TestConverter_Convert_Slice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		separator string
		target    any
		check     func(t *testing.T, target any)
		wantErr   bool
	}{
		{
			name:      "int slice",
			value:     "1,2,3,4,5",
			separator: ",",
			target:    &[]int{},
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*[]int)
				require.True(t, ok)
				assert.Equal(t, []int{1, 2, 3, 4, 5}, *v)
			},
		},
		{
			name:      "string slice",
			value:     "a,b,c",
			separator: ",",
			target:    &[]string{},
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*[]string)
				require.True(t, ok)
				assert.Equal(t, []string{"a", "b", "c"}, *v)
			},
		},
		{
			name:      "string slice with spaces",
			value:     "a, b, c",
			separator: ",",
			target:    &[]string{},
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*[]string)
				require.True(t, ok)
				assert.Equal(t, []string{"a", "b", "c"}, *v)
			},
		},
		{
			name:      "empty slice",
			value:     "",
			separator: ",",
			target:    &[]int{},
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*[]int)
				require.True(t, ok)
				assert.Equal(t, []int{}, *v)
			},
		},
		{
			name:      "custom separator",
			value:     "1;2;3",
			separator: ";",
			target:    &[]int{},
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*[]int)
				require.True(t, ok)
				assert.Equal(t, []int{1, 2, 3}, *v)
			},
		},
		{
			name:      "bool slice",
			value:     "true,false,yes,no",
			separator: ",",
			target:    &[]bool{},
			check: func(t *testing.T, target any) {
				t.Helper()

				v, ok := target.(*[]bool)
				require.True(t, ok)
				assert.Equal(t, []bool{true, false, true, false}, *v)
			},
		},
		{
			name:      "invalid element",
			value:     "1,invalid,3",
			separator: ",",
			target:    &[]int{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := reflect.ValueOf(tt.target).Elem()
			c := &typeconv.Converter{
				SliceSeparator: tt.separator,
				TimeLayout:     time.RFC3339,
			}
			err := c.Convert(target, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				tt.check(t, tt.target)
			}
		})
	}
}

func TestConverter_Convert_NotSettable(t *testing.T) {
	t.Parallel()

	var result int
	// Don't use Elem(), so it's not settable
	target := reflect.ValueOf(result)

	c := typeconv.New()
	err := c.Convert(target, "42")
	require.Error(t, err)
	assert.ErrorIs(t, err, typeconv.ErrUnsupportedType)
}

func TestConverter_Convert_UnsupportedType(t *testing.T) {
	t.Parallel()

	type CustomStruct struct {
		Field string
	}

	var result CustomStruct

	target := reflect.ValueOf(&result).Elem()

	c := typeconv.New()
	err := c.Convert(target, "test")
	require.Error(t, err)
	assert.ErrorIs(t, err, typeconv.ErrUnsupportedType)
}

func TestConvertTo(t *testing.T) {
	t.Parallel()

	t.Run("int", func(t *testing.T) {
		t.Parallel()

		result, err := typeconv.ConvertTo[int]("42")
		require.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("string", func(t *testing.T) {
		t.Parallel()

		result, err := typeconv.ConvertTo[string]("hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("bool", func(t *testing.T) {
		t.Parallel()

		result, err := typeconv.ConvertTo[bool]("true")
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("duration", func(t *testing.T) {
		t.Parallel()

		result, err := typeconv.ConvertTo[time.Duration]("5s")
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, result)
	})

	t.Run("error", func(t *testing.T) {
		t.Parallel()

		_, err := typeconv.ConvertTo[int]("invalid")
		assert.Error(t, err)
	})
}

func TestMustConvertTo(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		result := typeconv.MustConvertTo[int]("42")
		assert.Equal(t, 42, result)
	})

	t.Run("panic", func(t *testing.T) {
		t.Parallel()

		assert.Panics(t, func() {
			typeconv.MustConvertTo[int]("invalid")
		})
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	c := typeconv.New()
	assert.NotNil(t, c)
	assert.Equal(t, ",", c.SliceSeparator)
	assert.Equal(t, time.RFC3339, c.TimeLayout)
}

func TestDefault(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, typeconv.Default)
	assert.Equal(t, ",", typeconv.Default.SliceSeparator)
	assert.Equal(t, time.RFC3339, typeconv.Default.TimeLayout)
}
