package api_keys

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewDeleteAPIKeyParams creates a new DeleteAPIKeyParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewDeleteAPIKeyParams() *DeleteAPIKeyParams {
	return &DeleteAPIKeyParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewDeleteAPIKeyParamsWithTimeout creates a new DeleteAPIKeyParams object
// with the ability to set a timeout on a request.
func NewDeleteAPIKeyParamsWithTimeout(timeout time.Duration) *DeleteAPIKeyParams {
	return &DeleteAPIKeyParams{
		timeout: timeout,
	}
}

// NewDeleteAPIKeyParamsWithContext creates a new DeleteAPIKeyParams object
// with the ability to set a context for a request.
func NewDeleteAPIKeyParamsWithContext(ctx context.Context) *DeleteAPIKeyParams {
	return &DeleteAPIKeyParams{
		Context: ctx,
	}
}

// NewDeleteAPIKeyParamsWithHTTPClient creates a new DeleteAPIKeyParams object
// with the ability to set a custom HTTPClient for a request.
func NewDeleteAPIKeyParamsWithHTTPClient(client *http.Client) *DeleteAPIKeyParams {
	return &DeleteAPIKeyParams{
		HTTPClient: client,
	}
}

/*
DeleteAPIKeyParams contains all the parameters to send to the API endpoint

	for the delete api key operation.

	Typically these are written to a http.Request.
*/
type DeleteAPIKeyParams struct {

	// ID.
	ID string `json:"id"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the delete api key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteAPIKeyParams) WithDefaults() *DeleteAPIKeyParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the delete api key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteAPIKeyParams) SetDefaults() {
	val := DeleteAPIKeyParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the delete api key params
func (o *DeleteAPIKeyParams) WithTimeout(timeout time.Duration) *DeleteAPIKeyParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the delete api key params
func (o *DeleteAPIKeyParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the delete api key params
func (o *DeleteAPIKeyParams) WithContext(ctx context.Context) *DeleteAPIKeyParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the delete api key params
func (o *DeleteAPIKeyParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the delete api key params
func (o *DeleteAPIKeyParams) WithHTTPClient(client *http.Client) *DeleteAPIKeyParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the delete api key params
func (o *DeleteAPIKeyParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithID adds the id to the delete api key params
func (o *DeleteAPIKeyParams) WithID(id string) *DeleteAPIKeyParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the delete api key params
func (o *DeleteAPIKeyParams) SetID(id string) {
	o.ID = id
}

// WriteToRequest writes these params to a swagger request
func (o *DeleteAPIKeyParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param id
	if err := r.SetPathParam("id", o.ID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
