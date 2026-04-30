package dispatch

// SetDirExists replaces the dirExists function for testing and returns a restore function.
func SetDirExists(fn func(string) bool) func() {
	orig := dirExists
	dirExists = fn
	return func() { dirExists = orig }
}

// SetMarshalMeta replaces the marshalMeta function for testing and returns a restore function.
func SetMarshalMeta(fn func(any) ([]byte, error)) func() {
	orig := marshalMeta
	marshalMeta = fn
	return func() { marshalMeta = orig }
}
