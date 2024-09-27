package auth

import (
	"context"
	"net/http"
	"net/url"

	token "github.com/equinor/radix-api/api/utils/authn"
	"github.com/equinor/radix-common/models"
	radixhttp "github.com/equinor/radix-common/net/http"
	"github.com/rs/zerolog/log"
	"github.com/urfave/negroni/v3"
)

type ctxUserKey struct{}
type ctxImpersonationKey struct{}

func CreateAuthenticationMiddleware(issuer, audience string) negroni.HandlerFunc {
	issuerUrl, err := url.Parse(issuer)
	if err != nil {
		log.Fatal().Err(err).Msg("Error parsing issuer url")
	}

	// Set up the validator.
	jwtValidator, err := token.NewValidator(issuerUrl, audience)
	if err != nil {
		log.Fatal().Err(err).Msg("Error creating JWT validator")
	}

	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		ctx := r.Context()
		logger := log.Ctx(ctx)
		if r.Header.Get("authorization") == "" {
			next(w, r)
			return
		}

		token, err := radixhttp.GetBearerTokenFromHeader(r)
		if err != nil {
			logger.Warn().Err(err).Msg("authentication error")
			if err = radixhttp.ErrorResponse(w, r, err); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}
		principal, err := jwtValidator.ValidateToken(ctx, token)
		if err != nil {
			logger.Warn().Err(err).Msg("authentication error")
			if err = radixhttp.ErrorResponse(w, r, err); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}
		logContext := log.Ctx(ctx).With().Str("user", principal.Subject())

		impersonation, err := radixhttp.GetImpersonationFromHeader(r)
		if err != nil {
			logger.Warn().Err(err).Msg("authorization error")
			if err = radixhttp.ErrorResponse(w, r, radixhttp.UnexpectedError("Problems impersonating", err)); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}
		if impersonation.PerformImpersonation() {
			logContext = logContext.Str("impersonate_user", impersonation.User).Strs("impersonate_groups", impersonation.Groups)
		}

		ctx = context.WithValue(ctx, ctxUserKey{}, principal)
		ctx = context.WithValue(ctx, ctxImpersonationKey{}, impersonation)
		ctx = logContext.Logger().WithContext(ctx)
		r = r.WithContext(ctx)

		next(w, r)
	}
}

func GetToken(ctx context.Context) token.TokenPrincipal {
	val, ok := ctx.Value(ctxUserKey{}).(token.TokenPrincipal)

	if !ok {
		return token.NewAnonymousPrincipal()
	}

	return val
}

func GetImpersonation(ctx context.Context) models.Impersonation {
	if val, ok := ctx.Value(ctxImpersonationKey{}).(models.Impersonation); ok {
		return val
	}

	return models.Impersonation{}
}

func CreateAuthorizeRequiredMiddleware() negroni.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		logger := log.Ctx(r.Context())
		user := GetToken(r.Context())

		if !user.IsAuthenticated() {
			logger.Warn().Msg("authorization error")
			if err := radixhttp.ErrorResponse(w, r, radixhttp.ForbiddenError("Authorization is required")); err != nil {
				logger.Err(err).Msg("failed to write response")
			}
			return
		}

		next(w, r)
		return
	}
}
