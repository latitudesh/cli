package api_keys

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"

	"github.com/latitudesh/lsh/models"
)

// NewPostAPIKeyParams creates a new PostAPIKeyParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewPostAPIKeyParams() *PostAPIKeyParams {
	return &PostAPIKeyParams{
		timeout: cr.DefaultTimeout,
		Body: &models.CreateAPIKey{
			Data: &models.CreateAPIKeyData{
				Type:       &apiKeysType,
				Attributes: &models.CreateAPIKeyDataAttributes{APIVersion: "2023-06-01"},
			},
		},
	}
}

// NewPostAPIKeyParamsWithTimeout creates a new PostAPIKeyParams object
// with the ability to set a timeout on a request.
func NewPostAPIKeyParamsWithTimeout(timeout time.Duration) *PostAPIKeyParams {
	return &PostAPIKeyParams{
		timeout: timeout,
	}
}

// NewPostAPIKeyParamsWithContext creates a new PostAPIKeyParams object
// with the ability to set a context for a request.
func NewPostAPIKeyParamsWithContext(ctx context.Context) *PostAPIKeyParams {
	return &PostAPIKeyParams{
		Context: ctx,
	}
}

// NewPostAPIKeyParamsWithHTTPClient creates a new PostAPIKeyParams object
// with the ability to set a custom HTTPClient for a request.
func NewPostAPIKeyParamsWithHTTPClient(client *http.Client) *PostAPIKeyParams {
	return &PostAPIKeyParams{
		HTTPClient: client,
	}
}

/*
PostAPIKeyParams contains all the parameters to send to the API endpoint

	for the post api key operation.

	Typically these are written to a http.Request.
*/
type PostAPIKeyParams struct {

	// Body.
	Body *models.CreateAPIKey

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the post api key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PostAPIKeyParams) WithDefaults() *PostAPIKeyParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the post api key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *PostAPIKeyParams) SetDefaults() {
	val := PostAPIKeyParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the post api key params
func (o *PostAPIKeyParams) WithTimeout(timeout time.Duration) *PostAPIKeyParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the post api key params
func (o *PostAPIKeyParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the post api key params
func (o *PostAPIKeyParams) WithContext(ctx context.Context) *PostAPIKeyParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the post api key params
func (o *PostAPIKeyParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the post api key params
func (o *PostAPIKeyParams) WithHTTPClient(client *http.Client) *PostAPIKeyParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the post api key params
func (o *PostAPIKeyParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the post api key params
func (o *PostAPIKeyParams) WithBody(body *models.CreateAPIKey) *PostAPIKeyParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the post api key params
func (o *PostAPIKeyParams) SetBody(body *models.CreateAPIKey) {
	o.Body = body
}

// WriteToRequest writes these params to a swagger request
func (o *PostAPIKeyParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Body != nil {
		if err := r.SetBodyParam(o.Body); err != nil {
			return err
		}
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
