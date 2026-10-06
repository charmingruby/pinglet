package validator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

type Validator struct {
	validator *validator.Validate
}

type ctxKey struct{}

func New() *Validator {
	return &Validator{
		validator: validator.New(),
	}
}

func (v *Validator) Validate(s any) error {
	err := v.validator.Struct(s)

	if err == nil {
		return nil
	}

	errs := v.unwrapValidationErr(err)

	return errors.New(strings.Join(errs, ", "))
}

func WithValidator(ctx context.Context, val *Validator) context.Context {
	return context.WithValue(ctx, ctxKey{}, val)
}

func ValidatorFromContext(ctx context.Context) *Validator {
	val, ok := ctx.Value(ctxKey{}).(*Validator)
	if !ok {
		return New()
	}

	return val
}

func (v *Validator) unwrapValidationErr(err error) []string {
	var valErr *validator.ValidationErrors
	if !errors.As(err, &valErr) {
		return []string{err.Error()}
	}

	errs := *valErr

	reasonsWrapper := make([]string, 0, len(errs))

	for _, vErr := range errs {
		reason := fmt.Sprintf("field `%s` does not satisfy %s rule", vErr.Field(), vErr.Tag())

		reasonsWrapper = append(reasonsWrapper, reason)
	}

	return reasonsWrapper
}
