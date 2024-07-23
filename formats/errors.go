package formats

type fileError struct {
	filename string
	error
}

func NewFileError(filename string, err error) fileError {
	return fileError{
		filename: filename,
		error:    err,
	}
}
