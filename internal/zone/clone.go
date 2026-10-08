package zone

import (
	"reflect"
	"unsafe"
)

// clone returns a deep copy of v: nothing reachable from the copy (slices,
// maps, pointers, through structs, arrays and interfaces, exported fields
// or not) is shared with v. It works off reflection so model fields added
// to Zone and Shape are copied with no change here. Nil slices and maps
// stay nil, so a clone is reflect.DeepEqual to its source. Cycles are not
// supported (the model has none).
func clone[T any](v T) T {
	src := reflect.ValueOf(&v).Elem()
	dst := reflect.New(src.Type()).Elem()
	cloneInto(dst, src)
	return dst.Interface().(T)
}

// cloneInto deep-copies src into dst, a settable value of the same type.
func cloneInto(dst, src reflect.Value) {
	switch src.Kind() {
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := range src.Len() {
			cloneInto(dst.Index(i), src.Index(i))
		}
	case reflect.Array:
		for i := range src.Len() {
			cloneInto(dst.Index(i), src.Index(i))
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		for it := src.MapRange(); it.Next(); {
			k := reflect.New(src.Type().Key()).Elem()
			cloneInto(k, it.Key())
			val := reflect.New(src.Type().Elem()).Elem()
			cloneInto(val, it.Value())
			dst.SetMapIndex(k, val)
		}
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		p := reflect.New(src.Type().Elem())
		cloneInto(p.Elem(), src.Elem())
		dst.Set(p)
	case reflect.Interface:
		if src.IsNil() {
			return
		}
		inner := reflect.New(src.Elem().Type()).Elem()
		cloneInto(inner, src.Elem())
		dst.Set(inner)
	case reflect.Struct:
		if !src.CanAddr() { // a map value or an interface's dynamic value
			tmp := reflect.New(src.Type()).Elem()
			tmp.Set(src)
			src = tmp
		}
		for i := range src.NumField() {
			cloneInto(settable(dst.Field(i)), settable(src.Field(i)))
		}
	default:
		dst.Set(src)
	}
}

// settable makes a field of an addressable struct usable even when it is
// unexported, which reflect otherwise refuses to read through or set.
func settable(f reflect.Value) reflect.Value {
	if f.CanSet() {
		return f
	}
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}
