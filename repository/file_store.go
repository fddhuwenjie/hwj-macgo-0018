package repository

import "errors"

func IsDurableSaveFailure(err error) bool { return errors.Is(err, ErrStoreClosed) }
