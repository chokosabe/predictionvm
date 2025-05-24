//go:generate canoto --internal $GOFILE

// Canoto provides common functionality required for reading and writing the
// canoto format.
package canoto

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"math"
	"math/bits"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"
	"unsafe"

	_ "embed"
)

const (
	Varint WireType = iota
	I64
	Len
	_ // SGROUP is deprecated and not supported
	_ // EGROUP is deprecated and not supported
	I32

	// SizeEnum8 indicates either an int8 or uint8.
	SizeEnum8 SizeEnum = 1
	// SizeEnum16 indicates either an int16 or uint16.
	SizeEnum16 SizeEnum = 2
	// SizeEnum32 indicates either an int32 or uint32.
	SizeEnum32 SizeEnum = 3
	// SizeEnum64 indicates either an int64 or uint64.
	SizeEnum64 SizeEnum = 4

	// SizeFint32 is the size of a 32-bit fixed size integer in bytes.
	SizeFint32 = 4
	// SizeFint64 is the size of a 64-bit fixed size integer in bytes.
	SizeFint64 = 8
	// SizeBool is the size of a boolean in bytes.
	SizeBool = 1

	// MaxFieldNumber is the maximum field number allowed to be used in a Tag.
	MaxFieldNumber = 1<<29 - 1

	// Version is the current version of the canoto library.
	Version = "v0.15.0"

	wireTypeLength = 3
	wireTypeMask   = 0x07

	falseByte        = 0
	trueByte         = 1
	continuationMask = 0x80
)

var (
	//go:embed canoto.go
	code string

	// Code is the actual golang code for this library; including this comment.
	//
	// This variable is not used internally, so the compiler is smart enough to
	// omit this value from the binary if the user of this library does not
	// utilize this variable; at least at the time of writing.
	//
	// This can be used during codegen to generate this library.
	Code, _ = strings.CutPrefix(code, "//go:generate canoto --internal $GOFILE\n\n")

	// GeneratedCode is the actual auto-generated golang code for this library.
	//
	// This variable is not used internally, so the compiler is smart enough to
	// omit this value from the binary if the user of this library does not
	// utilize this variable; at least at the time of writing.
	//
	// This can be used during codegen to generate this library.
	//
	//go:embed canoto.canoto.go
	GeneratedCode string

	ErrInvalidFieldOrder  = errors.New("invalid field order")
	ErrUnexpectedWireType = errors.New("unexpected wire type")
	ErrDuplicateOneOf     = errors.New("duplicate oneof field")
	ErrInvalidLength      = errors.New("decoded length is invalid")
	ErrZeroValue          = errors.New("zero value")
	ErrUnknownField       = errors.New("unknown field")
	ErrPaddedZeroes       = errors.New("padded zeroes")

	ErrInvalidRecursiveDepth = errors.New("invalid recursive depth")
	ErrUnknownFieldType      = errors.New("unknown field type")
	ErrUnexpectedFieldSize   = errors.New("unexpected field size")
	ErrInvalidFieldType      = errors.New("invalid field type")

	ErrOverflow        = errors.New("overflow")
	ErrInvalidWireType = errors.New("invalid wire type")
	ErrInvalidBool     = errors.New("decoded bool is neither true nor false")
	ErrStringNotUTF8   = errors.New("decoded string is not UTF-8")

	_ json.Marshaler = Any{}
)

type (
	Int interface {
		~int8 | ~int16 | ~int32 | ~int64
	}
	Uint interface {
		~uint8 | ~uint16 | ~uint32 | ~uint64
	}
	integer interface{ Int | Uint }
	Int32   interface{ ~int32 | ~uint32 }
	Int64   interface{ ~int64 | ~uint64 }
	Bytes   interface{ ~string | ~[]byte }

	// Message defines a type that can be a stand-alone Canoto message.
	Message interface {
		Field
		// MarshalCanoto returns the Canoto representation of this message.
		//
		// It is assumed that this message is ValidCanoto.
		MarshalCanoto() []byte
		// UnmarshalCanoto unmarshals a Canoto-encoded byte slice into the
		// message.
		UnmarshalCanoto(bytes []byte) error
	}

	// Field defines a type that can be included inside of a Canoto message.
	Field interface {
		// CanotoSpec returns the specification of this canoto message.
		//
		// If there is not a valid specification of this type, it returns nil.
		CanotoSpec(types ...reflect.Type) *Spec
		// MarshalCanotoInto writes the field into a canoto.Writer and returns
		// the resulting canoto.Writer.
		//
		// It is assumed that CalculateCanotoCache has been called since the
		// last modification to this field.
		//
		// It is assumed that this field is ValidCanoto.
		MarshalCanotoInto(w Writer) Writer
		// CalculateCanotoCache populates internal caches based on the current
		// values in the struct.
		CalculateCanotoCache()
		// CachedCanotoSize returns the previously calculated size of the Canoto
		// representation from CalculateCanotoCache.
		//
		// If CalculateCanotoCache has not yet been called, or the field has
		// been modified since the last call to CalculateCanotoCache, the
		// returned size may be incorrect.
		CachedCanotoSize() uint64
		// UnmarshalCanotoFrom populates the field from a canoto.Reader.
		UnmarshalCanotoFrom(r Reader) error
		// ValidCanoto validates that the field can be correctly marshaled into
		// the Canoto format.
		ValidCanoto() bool
	}

	// FieldPointer is a pointer to a concrete Field value T.
	//
	// This type must be used when implementing a value for a generic Field.
	FieldPointer[T any] interface {
		Field
		*T
	}

	// FieldMaker is a Field that can create a new value of type T.
	//
	// The returned value must be able to be unmarshaled into.
	//
	// This type can be used when implementing a generic Field. However, if T is
	// an interface, it is possible for generated code to compile and panic at
	// runtime.
	FieldMaker[T any] interface {
		Field
		MakeCanoto() T
	}

	// WireType represents the Proto wire description of a field. Within Proto
	// it is used to provide forwards compatibility. For Canoto, it exists to
	// provide compatibility with Proto.
	WireType byte

	// Reader contains all the state needed to unmarshal a Canoto type.
	//
	// The functions in this package are not methods on the Reader type to
	// enable the usage of generics.
	Reader struct {
		B      []byte
		Unsafe bool
		// Context is a user-defined value that can be used to pass additional
		// state during the unmarshaling process.
		Context any
	}

	// Writer contains all the state needed to marshal a Canoto type.
	//
	// The functions in this package are not methods on the Writer type to
	// enable the usage of generics.
	Writer struct {
		B []byte
	}

	// Spec is the specification of a Canoto message.
	//
	// Given a message specification, [Unmarshal] can be used to parse bytes
	// into an [Any].
	//
	// Spec is itself a message, to allow for implementations of universal
	// canoto message interpreters.
	Spec struct {
		Name   string      `canoto:"string,1"         json:"name"`
		Fields []FieldType `canoto:"repeated value,2" json:"fields"`

		canotoData canotoData_Spec
	}

	// FieldType is the specification of a field in a Canoto message.
	FieldType struct {
		FieldNumber    uint32   `canoto:"uint,1"          json:"fieldNumber"`
		Name           string   `canoto:"string,2"        json:"name"`
		FixedLength    uint64   `canoto:"uint,3"          json:"fixedLength,omitempty"`
		Repeated       bool     `canoto:"bool,4"          json:"repeated,omitempty"`
		OneOf          string   `canoto:"string,5"        json:"oneOf,omitempty"`
		TypeInt        SizeEnum `canoto:"uint,6,Type"     json:"typeInt,omitempty"`        // can be any of 8, 16, 32, or 64.
		TypeUint       SizeEnum `canoto:"uint,7,Type"     json:"typeUint,omitempty"`       // can be any of 8, 16, 32, or 64.
		TypeFixedInt   SizeEnum `canoto:"uint,8,Type"     json:"typeFixedInt,omitempty"`   // can be either 32 or 64.
		TypeFixedUint  SizeEnum `canoto:"uint,9,Type"     json:"typeFixedUint,omitempty"`  // can be either 32 or 64.
		TypeBool       bool     `canoto:"bool,10,Type"    json:"typeBool,omitempty"`       // can only be true.
		TypeString     bool     `canoto:"bool,11,Type"    json:"typeString,omitempty"`     // can only be true.
		TypeBytes      bool     `canoto:"bool,12,Type"    json:"typeBytes,omitempty"`      // can only be true.
		TypeFixedBytes uint64   `canoto:"uint,13,Type"    json:"typeFixedBytes,omitempty"` // length of the fixed bytes.
		TypeRecursive  uint64   `canoto:"uint,14,Type"    json:"typeRecursive,omitempty"`  // depth of the recursion.
		TypeMessage    *Spec    `canoto:"pointer,15,Type" json:"typeMessage,omitempty"`

		canotoData canotoData_FieldType
	}

	// SizeEnum indicate the size of an integer type in canoto specifications.
	SizeEnum uint8

	// Any is a generic representation of a Canoto message.
	Any struct {
		Fields []AnyField
	}

	// AnyField is a generic representation of a field in a Canoto message.
	AnyField struct {
		Name string

		// Value is the value of the field.
		//
		// It can be any of the following types:
		//   - int64,  []int64
		//   - uint64, []uint64
		//   - bool,   []bool
		//   - string, []string
		//   - []byte, [][]byte
		//   - Any,    []Any
		Value any
	}
)

func (w WireType) IsValid() bool {
	switch w {
	case Varint, I64, Len, I32:
		return true
	default:
		return false
	}
}

func (w WireType) String() string {
	switch w {
	case Varint:
		return "Varint"
	case I64:
		return "I64"
	case Len:
		return "Len"
	case I32:
		return "I32"
	default:
		return "Invalid"
	}
}

func (s SizeEnum) FixedWireType() (WireType, bool) {
	switch s {
	case SizeEnum32:
		return I32, true
	case SizeEnum64:
		return I64, true
	default:
		return 0, false
	}
}

func (s SizeEnum) NumBytes() (uint64, bool) {
	switch s {
	case SizeEnum8:
		return 1, true
	case SizeEnum16:
		return 2, true
	case SizeEnum32:
		return 4, true
	case SizeEnum64:
		return 8, true
	default:
		return 0, false
	}
}

// HasNext returns true if there are more bytes to read.
func HasNext(r *Reader) bool {
	return len(r.B) > 0
}

// Append writes unprefixed bytes to the writer.
func Append[T Bytes](w *Writer, v T) {
	w.B = append(w.B, v...)
}

// Tag calculates the tag for a field number and wire type.
//
// This function should not typically be used during marshaling, as tags can be
// precomputed.
func Tag(fieldNumber uint32, wireType WireType) []byte {
	w := Writer{}
	AppendUint(&w, fieldNumber<<wireTypeLength|uint32(wireType))
	return w.B
}

// ReadTag reads the next field number and wire type from the reader.
func ReadTag(r *Reader) (uint32, WireType, error) {
	var val uint32
	if err := ReadUint(r, &val); err != nil {
		return 0, 0, err
	}

	wireType := WireType(val & wireTypeMask)
	if !wireType.IsValid() {
		return 0, 0, ErrInvalidWireType
	}

	return val >> wireTypeLength, wireType, nil
}

// SizeUint calculates the size of an unsigned integer when encoded as a varint.
func SizeUint[T Uint](v T) uint64 {
	if v == 0 {
		return 1
	}
	return uint64(bits.Len64(uint64(v))+6) / 7 //#nosec G115 // False positive
}

// CountInts counts the number of varints that are encoded in bytes.
func CountInts(bytes []byte) uint64 {
	var count uint64
	for _, b := range bytes {
		if b < continuationMask {
			count++
		}
	}
	return count
}

// ReadUint reads a varint encoded unsigned integer from the reader.
func ReadUint[T Uint](r *Reader, v *T) error {
	val, bytesRead := binary.Uvarint(r.B)
	switch {
	case bytesRead == 0:
		return io.ErrUnexpectedEOF
	case bytesRead < 0 || uint64(T(val)) != val:
		return ErrOverflow
	// To ensure decoding is canonical, we check for padded zeroes in the
	// varint.
	// The last byte of the varint includes the most significant bits.
	// If the last byte is 0, then the number should have been encoded more
	// efficiently by removing this zero.
	case bytesRead > 1 && r.B[bytesRead-1] == 0x00:
		return ErrPaddedZeroes
	default:
		r.B = r.B[bytesRead:]
		*v = T(val)
		return nil
	}
}

// AppendUint writes an unsigned integer to the writer as a varint.
func AppendUint[T Uint](w *Writer, v T) {
	w.B = binary.AppendUvarint(w.B, uint64(v))
}

// SizeInt calculates the size of an integer when zigzag encoded as a varint.
func SizeInt[T Int](v T) uint64 {
	if v == 0 {
		return 1
	}

	var uv uint64
	if v > 0 {
		uv = uint64(v) << 1
	} else {
		uv = ^uint64(v)<<1 | 1
	}
	return uint64(bits.Len64(uv)+6) / 7 //#nosec G115 // False positive
}

// ReadInt reads a zigzag encoded integer from the reader.
func ReadInt[T Int](r *Reader, v *T) error {
	var largeVal uint64
	if err := ReadUint(r, &largeVal); err != nil {
		return err
	}

	uVal := largeVal >> 1
	val := T(uVal)
	// If T is an int32, it's possible that some bits were truncated during the
	// cast. In this case, casting back to uint64 would result in a different
	// value.
	if uint64(val) != uVal {
		return ErrOverflow
	}

	if largeVal&1 != 0 {
		val = ^val
	}
	*v = val
	return nil
}

// AppendInt writes an integer to the writer as a zigzag encoded varint.
func AppendInt[T Int](w *Writer, v T) {
	if v >= 0 {
		w.B = binary.AppendUvarint(w.B, uint64(v)<<1)
	} else {
		w.B = binary.AppendUvarint(w.B, ^uint64(v)<<1|1)
	}
}

// ReadFint32 reads a 32-bit fixed size integer from the reader.
func ReadFint32[T Int32](r *Reader, v *T) error {
	if len(r.B) < SizeFint32 {
		return io.ErrUnexpectedEOF
	}

	*v = T(binary.LittleEndian.Uint32(r.B))
	r.B = r.B[SizeFint32:]
	return nil
}

// AppendFint32 writes a 32-bit fixed size integer to the writer.
func AppendFint32[T Int32](w *Writer, v T) {
	w.B = binary.LittleEndian.AppendUint32(w.B, uint32(v))
}

// ReadFint64 reads a 64-bit fixed size integer from the reader.
func ReadFint64[T Int64](r *Reader, v *T) error {
	if len(r.B) < SizeFint64 {
		return io.ErrUnexpectedEOF
	}

	*v = T(binary.LittleEndian.Uint64(r.B))
	r.B = r.B[SizeFint64:]
	return nil
}

// AppendFint64 writes a 64-bit fixed size integer to the writer.
func AppendFint64[T Int64](w *Writer, v T) {
	w.B = binary.LittleEndian.AppendUint64(w.B, uint64(v))
}

// ReadBool reads a boolean from the reader.
func ReadBool[T ~bool](r *Reader, v *T) error {
	switch {
	case len(r.B) < SizeBool:
		return io.ErrUnexpectedEOF
	case r.B[0] > trueByte:
		return ErrInvalidBool
	default:
		*v = r.B[0] == trueByte
		r.B = r.B[SizeBool:]
		return nil
	}
}

// AppendBool writes a boolean to the writer.
func AppendBool[T ~bool](w *Writer, b T) {
	if b {
		w.B = append(w.B, trueByte)
	} else {
		w.B = append(w.B, falseByte)
	}
}

// SizeBytes calculates the size the length-prefixed bytes would take if
// written.
func SizeBytes[T Bytes](v T) uint64 {
	length := uint64(len(v))
	return SizeUint(length) + length
}

// CountBytes counts the consecutive number of length-prefixed fields with the
// given tag.
func CountBytes(bytes []byte, tag string) (uint64, error) {
	var (
		r     = Reader{B: bytes}
		count uint64
	)
	for HasPrefix(r.B, tag) {
		r.B = r.B[len(tag):]
		var length uint64
		if err := ReadUint(&r, &length); err != nil {
			return 0, err
		}
		if length > uint64(len(r.B)) {
			return 0, io.ErrUnexpectedEOF
		}
		r.B = r.B[length:]
		count++
	}
	return count, nil
}

// HasPrefix returns true if the bytes start with the given prefix.
func HasPrefix(bytes []byte, prefix string) bool {
	return len(bytes) >= len(prefix) && string(bytes[:len(prefix)]) == prefix
}

// ReadString reads a string from the reader. The string is verified to be valid
// UTF-8.
func ReadString[T ~string](r *Reader, v *T) error {
	var length uint64
	if err := ReadUint(r, &length); err != nil {
		return err
	}
	if length > uint64(len(r.B)) {
		return io.ErrUnexpectedEOF
	}

	bytes := r.B[:length]
	if !utf8.Valid(bytes) {
		return ErrStringNotUTF8
	}

	r.B = r.B[length:]
	if r.Unsafe {
		*v = T(unsafeString(bytes))
	} else {
		*v = T(bytes)
	}
	return nil
}

// ValidString returns true if it is valid to encode the provided string. A
// string is valid to encode if it is valid UTF-8.
func ValidString[T ~string](v T) bool {
	return utf8.ValidString(string(v))
}

// ReadBytes reads a byte slice from the reader.
func ReadBytes[T ~[]byte](r *Reader, v *T) error {
	var length uint64
	if err := ReadUint(r, &length); err != nil {
		return err
	}
	if length > uint64(len(r.B)) {
		return io.ErrUnexpectedEOF
	}

	bytes := r.B[:length]
	r.B = r.B[length:]
	if !r.Unsafe {
		bytes = slices.Clone(bytes)
	}
	*v = T(bytes)
	return nil
}

// AppendBytes writes a length-prefixed byte slice to the writer.
func AppendBytes[T Bytes](w *Writer, v T) {
	AppendUint(w, uint64(len(v)))
	w.B = append(w.B, v...)
}

// MakePointer creates a new pointer. It is equivalent to `new(T)`.
//
// This function is useful to use in auto-generated code, when the type of a
// variable is unknown. For example, if we have a variable `v` which we know to
// be a pointer, but we do not know the type of the pointer, we can use this
// function to leverage golang's type inference to create the new pointer.
func MakePointer[T any](_ *T) *T {
	return new(T)
}

// MakeSlice creates a new slice with the given length. It is equivalent to
// `make([]T, length)`.
//
// This function is useful to use in auto-generated code, when the type of a
// variable is unknown. For example, if we have a variable `v` which we know to
// be a slice, but we do not know the type of the elements, we can use this
// function to leverage golang's type inference to create the new slice.
func MakeSlice[T any](_ []T, length uint64) []T {
	return make([]T, length)
}

// MakeEntry returns the zero value of an element in the provided slice.
//
// This function is useful to use in auto-generated code, when the type of a
// variable is unknown. For example, if we have a variable `v` which we know to
// be a slice, but we do not know the type of the elements, we can use this
// function to leverage golang's type inference to create an element.
func MakeEntry[S ~[]E, E any](_ S) (_ E) {
	return
}

// MakeEntryNilPointer returns a nil pointer to an element in the provided
// slice.
//
// This function is useful to use in auto-generated code, when the type of a
// variable is unknown. For example, if we have a variable `v` which we know to
// be a slice, but we do not know the type of the elements, we can use this
// function to leverage golang's type inference to create the pointer.
func MakeEntryNilPointer[S ~[]E, E any](_ S) *E {
	return nil
}

// IsZero returns true if the value is the zero value for its type.
func IsZero[T comparable](v T) bool {
	var zero T
	return v == zero
}

// unsafeString converts a []byte to an unsafe string.
//
// Invariant: The input []byte must not be modified.
func unsafeString(b []byte) string {
	// avoid copying during the conversion
	return unsafe.String(unsafe.SliceData(b), len(b))
}

// Unmarshal unmarshals the given bytes into a message based on the
// specification.
//
// This function is significantly slower than calling UnmarshalCanoto on the
// type directly. This function should only be used when the concrete type can
// not be known ahead of time.
func Unmarshal(s *Spec, b []byte) (Any, error) {
	r := Reader{
		B: b,
	}
	return s.unmarshal(&r, nil)
}

// Marshal marshals the given message into bytes based on the specification.
//
// This function is significantly slower than calling MarshalCanoto on the type
// directly. This function should only be used when the concrete type can not be
// known ahead of time.
func Marshal(s *Spec, a Any) ([]byte, error) {
	// TODO: Implement size calculations to avoid quadratic marshalling
	// complexity.
	w := Writer{}
	err := s.marshal(&w, a, nil)
	return w.B, err
}

func (a Any) MarshalJSON() ([]byte, error) {
	var sb strings.Builder
	_, _ = sb.WriteString("{")
	for i, f := range a.Fields {
		if i > 0 {
			_, _ = sb.WriteString(",")
		}
		_, _ = sb.WriteString(`"`)
		_, _ = sb.WriteString(f.Name)
		_, _ = sb.WriteString(`":`)
		b, err := json.Marshal(f.Value)
		if err != nil {
			return nil, err
		}
		_, _ = sb.Write(b)
	}
	_, _ = sb.WriteString("}")
	return []byte(sb.String()), nil
}

// FieldTypeFromFint creates a FieldType from a fixed-length integer.
func FieldTypeFromFint[T integer](
	field T,
	fieldNumber uint32,
	name string,
	fixedLength uint64,
	repeated bool,
	oneOf string,
) FieldType {
	var (
		typeFixedInt  SizeEnum
		typeFixedUint SizeEnum
	)
	if isSigned[T]() {
		typeFixedInt = SizeOf(field)
	} else {
		typeFixedUint = SizeOf(field)
	}
	return FieldType{
		FieldNumber:   fieldNumber,
		Name:          name,
		FixedLength:   fixedLength,
		Repeated:      repeated,
		OneOf:         oneOf,
		TypeFixedInt:  typeFixedInt,
		TypeFixedUint: typeFixedUint,
	}
}

// FieldTypeFromField creates a FieldType from a field.
func FieldTypeFromField[T Field](
	field T,
	fieldNumber uint32,
	name string,
	fixedLength uint64,
	repeated bool,
	oneOf string,
	types []reflect.Type,
) FieldType {
	var (
		fieldType = reflect.TypeOf(field).Elem()

		typeBytes     bool
		typeRecursive uint64
		typeMessage   *Spec
	)
	if index := slices.Index(types, fieldType); index >= 0 {
		typeRecursive = uint64(len(types) - index) //#nosec G115 // False positive
	} else {
		typeMessage = field.CanotoSpec(types...)
		// If this does not have a valid spec, it is treated as bytes.
		if typeMessage == nil {
			typeBytes = true
		}
	}
	return FieldType{
		FieldNumber:   fieldNumber,
		Name:          name,
		FixedLength:   fixedLength,
		Repeated:      repeated,
		OneOf:         oneOf,
		TypeBytes:     typeBytes,
		TypeRecursive: typeRecursive,
		TypeMessage:   typeMessage,
	}
}

// isSigned returns true if the integer type is signed.
func isSigned[T integer]() bool {
	return ^T(0) < T(0)
}

// SizeOf returns the size of the integer type.
func SizeOf[T integer](_ T) SizeEnum {
	for i := range SizeEnum64 {
		bitLen := 1 << (i + 3)
		if T(1)<<bitLen == T(0) {
			return i + 1
		}
	}
	panic("unsupported integer size")
}

func (s *Spec) unmarshal(r *Reader, specs []*Spec) (Any, error) {
	specs = append(specs, s)
	var (
		minField uint32
		a        Any
		oneOfs   = make(map[string]struct{})
	)
	for HasNext(r) {
		fieldNumber, wireType, err := ReadTag(r)
		if err != nil {
			return Any{}, err
		}
		if fieldNumber < minField {
			return Any{}, ErrInvalidFieldOrder
		}

		fieldType, err := s.findFieldByNumber(fieldNumber)
		if err != nil {
			return Any{}, err
		}

		expectedWireType, err := fieldType.wireType()
		if err != nil {
			return Any{}, err
		}
		if wireType != expectedWireType {
			return Any{}, ErrUnexpectedWireType
		}

		if fieldType.OneOf != "" {
			if _, ok := oneOfs[fieldType.OneOf]; ok {
				return Any{}, ErrDuplicateOneOf
			}
			oneOfs[fieldType.OneOf] = struct{}{}
		}

		value, err := fieldType.unmarshal(r, specs)
		if err != nil {
			return Any{}, err
		}
		a.Fields = append(a.Fields, AnyField{
			Name:  fieldType.Name,
			Value: value,
		})

		minField = fieldNumber + 1
	}
	return a, nil
}

func (s *Spec) marshal(w *Writer, a Any, specs []*Spec) error {
	specs = append(specs, s)
	var minField uint32
	for _, f := range a.Fields {
		ft, err := s.findFieldByName(f.Name)
		if err != nil {
			return err
		}
		if ft.FieldNumber == 0 || ft.FieldNumber > MaxFieldNumber {
			return ErrUnknownField
		}
		if ft.FieldNumber < minField {
			return ErrInvalidFieldOrder
		}

		wireType, err := ft.wireType()
		if err != nil {
			return err
		}

		tag := Tag(ft.FieldNumber, wireType)
		Append(w, tag)

		if err := ft.marshal(w, f.Value, specs); err != nil {
			return err
		}

		minField = ft.FieldNumber + 1
	}
	return nil
}

func (s *Spec) findFieldByNumber(fieldNumber uint32) (*FieldType, error) {
	for i := range s.Fields {
		f := &s.Fields[i]
		if f.FieldNumber == fieldNumber {
			return f, nil
		}
	}
	return nil, ErrUnknownField
}

func (s *Spec) findFieldByName(name string) (*FieldType, error) {
	for i := range s.Fields {
		f := &s.Fields[i]
		if f.Name == name {
			return f, nil
		}
	}
	return nil, ErrUnknownField
}

func (f *FieldType) wireType() (WireType, error) {
	whichOneOf := f.CachedWhichOneOfType()
	switch whichOneOf {
	case 6, 7, 10:
		if f.Repeated {
			return Len, nil
		}
		return Varint, nil
	case 8:
		if f.Repeated {
			return Len, nil
		}
		w, ok := f.TypeFixedInt.FixedWireType()
		if !ok {
			return 0, ErrUnexpectedFieldSize
		}
		return w, nil
	case 9:
		if f.Repeated {
			return Len, nil
		}
		w, ok := f.TypeFixedUint.FixedWireType()
		if !ok {
			return 0, ErrUnexpectedFieldSize
		}
		return w, nil
	case 11, 12, 13, 14, 15:
		return Len, nil
	default:
		return 0, ErrUnknownFieldType
	}
}

func (f *FieldType) unmarshal(r *Reader, specs []*Spec) (any, error) {
	whichOneOf := f.CachedWhichOneOfType()
	unmarshal, ok := map[uint32]func(f *FieldType, r *Reader, specs []*Spec) (any, error){
		6:  (*FieldType).unmarshalInt,
		7:  (*FieldType).unmarshalUint,
		8:  (*FieldType).unmarshalFixedInt,
		9:  (*FieldType).unmarshalFixedUint,
		10: (*FieldType).unmarshalBool,
		11: (*FieldType).unmarshalString,
		12: (*FieldType).unmarshalBytes,
		13: (*FieldType).unmarshalFixedBytes,
		14: (*FieldType).unmarshalRecursive,
		15: (*FieldType).unmarshalSpec,
	}[whichOneOf]
	if !ok {
		return nil, ErrUnknownFieldType
	}
	value, err := unmarshal(f, r, specs)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (f *FieldType) marshal(w *Writer, value any, specs []*Spec) error {
	whichOneOf := f.CachedWhichOneOfType()
	marshal, ok := map[uint32]func(f *FieldType, w *Writer, value any, specs []*Spec) error{
		6:  (*FieldType).marshalInt,
		7:  (*FieldType).marshalUint,
		8:  (*FieldType).marshalFixedInt,
		9:  (*FieldType).marshalFixedUint,
		10: (*FieldType).marshalBool,
		11: (*FieldType).marshalString,
		12: (*FieldType).marshalBytes,
		13: (*FieldType).marshalBytes,
		14: (*FieldType).marshalRecursive,
		15: (*FieldType).marshalSpec,
	}[whichOneOf]
	if !ok {
		return ErrUnknownFieldType
	}
	return marshal(f, w, value, specs)
}

func (f *FieldType) unmarshalInt(r *Reader, _ []*Spec) (any, error) {
	return unmarshalPackedVarint(
		f,
		r,
		func(r *Reader) (int64, error) {
			switch f.TypeInt {
			case SizeEnum8:
				var v int8
				err := ReadInt(r, &v)
				return int64(v), err
			case SizeEnum16:
				var v int16
				err := ReadInt(r, &v)
				return int64(v), err
			case SizeEnum32:
				var v int32
				err := ReadInt(r, &v)
				return int64(v), err
			case SizeEnum64:
				var v int64
				err := ReadInt(r, &v)
				return v, err
			default:
				return 0, ErrUnexpectedFieldSize
			}
		},
	)
}

func (f *FieldType) marshalInt(w *Writer, value any, _ []*Spec) error {
	return marshalPacked(
		f,
		w,
		value,
		func(w *Writer, value int64) error {
			var (
				minimum int64
				maximum int64
			)
			switch f.TypeInt {
			case SizeEnum8:
				minimum, maximum = math.MinInt8, math.MaxInt8
			case SizeEnum16:
				minimum, maximum = math.MinInt16, math.MaxInt16
			case SizeEnum32:
				minimum, maximum = math.MinInt32, math.MaxInt32
			case SizeEnum64:
				minimum, maximum = math.MinInt64, math.MaxInt64
			default:
				return ErrUnexpectedFieldSize
			}
			if value < minimum || value > maximum {
				return ErrOverflow
			}
			AppendInt(w, value)
			return nil
		},
	)
}

func (f *FieldType) unmarshalUint(r *Reader, _ []*Spec) (any, error) {
	return unmarshalPackedVarint(
		f,
		r,
		func(r *Reader) (uint64, error) {
			switch f.TypeUint {
			case SizeEnum8:
				var v uint8
				err := ReadUint(r, &v)
				return uint64(v), err
			case SizeEnum16:
				var v uint16
				err := ReadUint(r, &v)
				return uint64(v), err
			case SizeEnum32:
				var v uint32
				err := ReadUint(r, &v)
				return uint64(v), err
			case SizeEnum64:
				var v uint64
				err := ReadUint(r, &v)
				return v, err
			default:
				return 0, ErrUnexpectedFieldSize
			}
		},
	)
}

func (f *FieldType) marshalUint(w *Writer, value any, _ []*Spec) error {
	return marshalPacked(
		f,
		w,
		value,
		func(w *Writer, value uint64) error {
			var maximum uint64
			switch f.TypeUint {
			case SizeEnum8:
				maximum = math.MaxUint8
			case SizeEnum16:
				maximum = math.MaxUint16
			case SizeEnum32:
				maximum = math.MaxUint32
			case SizeEnum64:
				maximum = math.MaxUint64
			default:
				return ErrUnexpectedFieldSize
			}
			if value > maximum {
				return ErrOverflow
			}
			AppendUint(w, value)
			return nil
		},
	)
}

func (f *FieldType) unmarshalFixedInt(r *Reader, _ []*Spec) (any, error) {
	return unmarshalPackedFixed(
		f,
		r,
		func(r *Reader) (int64, error) {
			switch f.TypeFixedInt {
			case SizeEnum32:
				var v int32
				err := ReadFint32(r, &v)
				return int64(v), err
			case SizeEnum64:
				var v int64
				err := ReadFint64(r, &v)
				return v, err
			default:
				return 0, ErrUnexpectedFieldSize
			}
		},
		f.TypeFixedInt,
	)
}

func (f *FieldType) marshalFixedInt(w *Writer, value any, _ []*Spec) error {
	return marshalPacked(
		f,
		w,
		value,
		func(w *Writer, value int64) error {
			switch f.TypeFixedInt {
			case SizeEnum32:
				if value < math.MinInt32 || value > math.MaxInt32 {
					return ErrOverflow
				}
				AppendFint32(w, int32(value))
			case SizeEnum64:
				AppendFint64(w, value)
			default:
				return ErrUnexpectedFieldSize
			}
			return nil
		},
	)
}

func (f *FieldType) unmarshalFixedUint(r *Reader, _ []*Spec) (any, error) {
	return unmarshalPackedFixed(
		f,
		r,
		func(r *Reader) (uint64, error) {
			switch f.TypeFixedUint {
			case SizeEnum32:
				var v uint32
				err := ReadFint32(r, &v)
				return uint64(v), err
			case SizeEnum64:
				var v uint64
				err := ReadFint64(r, &v)
				return v, err
			default:
				return 0, ErrUnexpectedFieldSize
			}
		},
		f.TypeFixedUint,
	)
}

func (f *FieldType) marshalFixedUint(w *Writer, value any, _ []*Spec) error {
	return marshalPacked(
		f,
		w,
		value,
		func(w *Writer, value uint64) error {
			switch f.TypeFixedUint {
			case SizeEnum32:
				if value > math.MaxUint32 {
					return ErrOverflow
				}
				AppendFint32(w, uint32(value))
			case SizeEnum64:
				AppendFint64(w, value)
			default:
				return ErrUnexpectedFieldSize
			}
			return nil
		},
	)
}

func (f *FieldType) unmarshalBool(r *Reader, _ []*Spec) (any, error) {
	return unmarshalPackedFixed(
		f,
		r,
		func(r *Reader) (bool, error) {
			var v bool
			err := ReadBool(r, &v)
			return v, err
		},
		1,
	)
}

func (f *FieldType) marshalBool(w *Writer, value any, _ []*Spec) error {
	return marshalPacked(
		f,
		w,
		value,
		func(w *Writer, value bool) error {
			AppendBool(w, value)
			return nil
		},
	)
}

func (f *FieldType) unmarshalString(r *Reader, _ []*Spec) (any, error) {
	return unmarshalUnpacked(
		f,
		r,
		func(msgBytes []byte) (string, bool, error) {
			if !utf8.Valid(msgBytes) {
				return "", false, ErrStringNotUTF8
			}
			return string(msgBytes), len(msgBytes) == 0, nil
		},
	)
}

func (f *FieldType) marshalString(w *Writer, value any, _ []*Spec) error {
	return marshalUnpacked(
		f,
		w,
		value,
		func(w *Writer, value string) error {
			AppendBytes(w, value)
			return nil
		},
	)
}

func (f *FieldType) unmarshalBytes(r *Reader, _ []*Spec) (any, error) {
	return unmarshalUnpacked(
		f,
		r,
		func(msgBytes []byte) ([]byte, bool, error) {
			return msgBytes, len(msgBytes) == 0, nil
		},
	)
}

func (f *FieldType) marshalBytes(w *Writer, value any, _ []*Spec) error {
	return marshalUnpacked(
		f,
		w,
		value,
		func(w *Writer, value []byte) error {
			AppendBytes(w, value)
			return nil
		},
	)
}

func (f *FieldType) unmarshalFixedBytes(r *Reader, _ []*Spec) (any, error) {
	// Read the first entry manually because the tag is already stripped.
	var length uint64
	if err := ReadUint(r, &length); err != nil {
		return nil, err
	}
	if length != f.TypeFixedBytes {
		return nil, ErrInvalidLength
	}
	if f.TypeFixedBytes > uint64(len(r.B)) {
		return nil, io.ErrUnexpectedEOF
	}

	msgBytes := r.B[:f.TypeFixedBytes]
	r.B = r.B[f.TypeFixedBytes:]

	if !f.Repeated {
		// If there is only one entry, return it.
		if isBytesEmpty(msgBytes) {
			return nil, ErrZeroValue
		}
		return slices.Clone(msgBytes), nil
	}

	// Count the number of additional entries after the first entry.
	expectedTag := Tag(f.FieldNumber, Len)

	count := f.FixedLength
	if count == 0 {
		countMinus1, err := CountBytes(r.B, string(expectedTag))
		if err != nil {
			return nil, err
		}
		count = countMinus1 + 1
	}

	values := make([][]byte, count)
	values[0] = slices.Clone(msgBytes)

	isZero := isBytesEmpty(msgBytes)

	// Read the rest of the entries, stripping the tag each time.
	for i := range count - 1 {
		if !HasPrefix(r.B, string(expectedTag)) {
			return nil, ErrUnknownField
		}
		r.B = r.B[len(expectedTag):]

		if err := ReadUint(r, &length); err != nil {
			return nil, err
		}
		if length != f.TypeFixedBytes {
			return nil, ErrInvalidLength
		}
		if f.TypeFixedBytes > uint64(len(r.B)) {
			return nil, io.ErrUnexpectedEOF
		}

		msgBytes := r.B[:f.TypeFixedBytes]
		r.B = r.B[f.TypeFixedBytes:]

		values[1+i] = slices.Clone(msgBytes)
		isZero = isZero && isBytesEmpty(msgBytes)
	}
	if f.FixedLength > 0 && isZero {
		return nil, ErrZeroValue
	}
	return values, nil
}

func (f *FieldType) unmarshalRecursive(r *Reader, specs []*Spec) (any, error) {
	spec, specs, err := f.recursiveSpec(specs)
	if err != nil {
		return nil, err
	}

	return unmarshalUnpacked(
		f,
		r,
		func(msgBytes []byte) (Any, bool, error) {
			if len(msgBytes) == 0 {
				return Any{}, true, nil
			}
			a, err := spec.unmarshal(
				&Reader{
					B: msgBytes,
				},
				specs,
			)
			return a, false, err
		},
	)
}

func (f *FieldType) marshalRecursive(w *Writer, value any, specs []*Spec) error {
	spec, specs, err := f.recursiveSpec(specs)
	if err != nil {
		return err
	}

	return marshalUnpacked(
		f,
		w,
		value,
		func(w *Writer, value Any) error {
			var tw Writer
			if err := spec.marshal(&tw, value, specs); err != nil {
				return err
			}
			AppendBytes(w, tw.B)
			return nil
		},
	)
}

func (f *FieldType) recursiveSpec(specs []*Spec) (*Spec, []*Spec, error) {
	numSpecs := uint64(len(specs))
	if f.TypeRecursive > numSpecs {
		return nil, nil, ErrInvalidRecursiveDepth
	}
	index := numSpecs - f.TypeRecursive
	spec := specs[index]
	specs = slices.Clone(specs[:index])
	return spec, specs, nil
}

func (f *FieldType) unmarshalSpec(r *Reader, specs []*Spec) (any, error) {
	return unmarshalUnpacked(
		f,
		r,
		func(msgBytes []byte) (Any, bool, error) {
			if len(msgBytes) == 0 {
				return Any{}, true, nil
			}
			a, err := f.TypeMessage.unmarshal(
				&Reader{
					B: msgBytes,
				},
				specs,
			)
			return a, false, err
		},
	)
}

func (f *FieldType) marshalSpec(w *Writer, value any, specs []*Spec) error {
	return marshalUnpacked(
		f,
		w,
		value,
		func(w *Writer, value Any) error {
			var tw Writer
			if err := f.TypeMessage.marshal(&tw, value, specs); err != nil {
				return err
			}
			AppendBytes(w, tw.B)
			return nil
		},
	)
}

func unmarshalPackedVarint[T comparable](
	f *FieldType,
	r *Reader,
	unmarshal func(r *Reader) (T, error),
) (any, error) {
	if !f.Repeated {
		// If there is only one entry, read it.
		value, err := unmarshal(r)
		if err != nil {
			return nil, err
		}
		if IsZero(value) {
			return nil, ErrZeroValue
		}
		return value, nil
	}

	// Read the full packed bytes.
	var msgBytes []byte
	if err := ReadBytes(r, &msgBytes); err != nil {
		return nil, err
	}

	count := f.FixedLength
	if count == 0 {
		if len(msgBytes) == 0 {
			return nil, ErrZeroValue
		}
		count = CountInts(msgBytes)
	}
	values := make([]T, count)
	r = &Reader{
		B: msgBytes,
	}
	isZero := true
	for i := range values {
		value, err := unmarshal(r)
		if err != nil {
			return nil, err
		}
		values[i] = value
		isZero = isZero && IsZero(value)
	}
	if HasNext(r) {
		return nil, ErrInvalidLength
	}
	if f.FixedLength > 0 && isZero {
		return nil, ErrZeroValue
	}
	return values, nil
}

func marshalPacked[T comparable](
	f *FieldType,
	w *Writer,
	value any,
	marshal func(w *Writer, value T) error,
) error {
	if !f.Repeated {
		// If there is only one entry, write it.
		v, ok := value.(T)
		if !ok {
			return ErrInvalidFieldType
		}
		return marshal(w, v)
	}

	vl, ok := value.([]T)
	if !ok {
		return ErrInvalidFieldType
	}

	var tw Writer
	for _, v := range vl {
		if err := marshal(&tw, v); err != nil {
			return err
		}
	}
	AppendBytes(w, tw.B)
	return nil
}

func unmarshalPackedFixed[T comparable](
	f *FieldType,
	r *Reader,
	unmarshal func(r *Reader) (T, error),
	sizeEnum SizeEnum,
) (any, error) {
	if !f.Repeated {
		// If there is only one entry, read it.
		value, err := unmarshal(r)
		if err != nil {
			return nil, err
		}
		if IsZero(value) {
			return nil, ErrZeroValue
		}
		return value, nil
	}

	// Read the full packed bytes.
	var msgBytes []byte
	if err := ReadBytes(r, &msgBytes); err != nil {
		return nil, err
	}

	count := f.FixedLength
	if count == 0 {
		numMsgBytes := uint64(len(msgBytes))
		if numMsgBytes == 0 {
			return nil, ErrZeroValue
		}

		size, ok := sizeEnum.NumBytes()
		if !ok {
			return nil, ErrUnexpectedFieldSize
		}
		if numMsgBytes%size != 0 {
			return nil, ErrInvalidLength
		}
		count = numMsgBytes / size
	}

	values := make([]T, count)
	r = &Reader{
		B: msgBytes,
	}
	isZero := true
	for i := range values {
		value, err := unmarshal(r)
		if err != nil {
			return nil, err
		}
		values[i] = value
		isZero = isZero && IsZero(value)
	}
	if HasNext(r) {
		return nil, ErrInvalidLength
	}
	if f.FixedLength > 0 && isZero {
		return nil, ErrZeroValue
	}
	return values, nil
}

func unmarshalUnpacked[T any](
	f *FieldType,
	r *Reader,
	unmarshal func([]byte) (T, bool, error),
) (any, error) {
	// Read the first entry manually because the tag is already stripped.
	var msgBytes []byte
	if err := ReadBytes(r, &msgBytes); err != nil {
		return nil, err
	}
	if !f.Repeated {
		// If there is only one entry, return it.
		value, isZero, err := unmarshal(msgBytes)
		if err != nil {
			return nil, err
		}
		if isZero {
			return nil, ErrZeroValue
		}
		return value, nil
	}

	// Count the number of additional entries after the first entry.
	expectedTag := Tag(f.FieldNumber, Len)

	count := f.FixedLength
	if count == 0 {
		countMinus1, err := CountBytes(r.B, string(expectedTag))
		if err != nil {
			return nil, err
		}
		count = countMinus1 + 1
	}

	values := make([]T, count)

	value, isZero, err := unmarshal(msgBytes)
	if err != nil {
		return nil, err
	}
	values[0] = value

	// Read the rest of the entries, stripping the tag each time.
	for i := range count - 1 {
		if !HasPrefix(r.B, string(expectedTag)) {
			return nil, ErrUnknownField
		}
		r.B = r.B[len(expectedTag):]

		if err := ReadBytes(r, &msgBytes); err != nil {
			return nil, err
		}

		var isFieldZero bool
		values[1+i], isFieldZero, err = unmarshal(msgBytes)
		if err != nil {
			return nil, err
		}
		isZero = isZero && isFieldZero
	}
	if f.FixedLength > 0 && isZero {
		return nil, ErrZeroValue
	}
	return values, nil
}

func marshalUnpacked[T any](
	f *FieldType,
	w *Writer,
	value any,
	marshal func(w *Writer, value T) error,
) error {
	if !f.Repeated {
		// If there is only one entry, write it.
		v, ok := value.(T)
		if !ok {
			return ErrInvalidFieldType
		}
		return marshal(w, v)
	}

	vl, ok := value.([]T)
	if !ok {
		return ErrInvalidFieldType
	}

	if len(vl) == 0 {
		return ErrInvalidLength
	}

	// Write the first entry manually because the tag is already written.
	if err := marshal(w, vl[0]); err != nil {
		return err
	}

	tag := Tag(f.FieldNumber, Len)
	for _, v := range vl[1:] {
		Append(w, tag)
		if err := marshal(w, v); err != nil {
			return err
		}
	}
	return nil
}

// isBytesEmpty returns true if the byte slice is all zeros.
func isBytesEmpty(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}
