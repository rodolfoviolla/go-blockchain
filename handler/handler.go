package handler

import "log"

func ErrorHandler[T interface{}](result T, errors ...error) T {
	if err, ok := interface{}(result).(error); ok && err != nil {
		log.Panic(err)
	}
	if len(errors) > 0 {
		if err := errors[0]; err != nil {
			log.Panic(err)
		}
	}
	return result
}