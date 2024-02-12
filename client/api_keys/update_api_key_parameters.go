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

// NewUpdateAPIKeyParams creates a new UpdateAPIKeyParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewUpdateAPIKeyParams() *UpdateAPIKeyParams {
	return &UpdateAPIKeyParams{
		timeout: cr.DefaultTimeout,
		Body: &models.UpdateAPIKey{
			Data: &models.UpdateAPIKeyData{
				Type:       &apiKeysType,
				Attributes: &models.UpdateAPIKeyDataAttributes{APIVersion: "2023-06-01"},
			},
		},
	}
}

// NewUpdateAPIKeyParamsWithTimeout creates a new UpdateAPIKeyParams object
// with the ability to set a timeout on a request.
func NewUpdateAPIKeyParamsWithTimeout(timeout time.Duration) *UpdateAPIKeyParams {
	return &UpdateAPIKeyParams{
		timeout: timeout,
	}
}

// NewUpdateAPIKeyParamsWithContext creates a new UpdateAPIKeyParams object
// with the ability to set a context for a request.
func NewUpdateAPIKeyParamsWithContext(ctx context.Context) *UpdateAPIKeyParams {
	return &UpdateAPIKeyParams{
		Context: ctx,
	}
}

// NewUpdateAPIKeyParamsWithHTTPClient creates a new UpdateAPIKeyParams object
// with the ability to set a custom HTTPClient for a request.
func NewUpdateAPIKeyParamsWithHTTPClient(client *http.Client) *UpdateAPIKeyParams {
	return &UpdateAPIKeyParams{
		HTTPClient: client,
	}
}

/*
UpdateAPIKeyParams contains all the parameters to send to the API endpoint

	for the update api key operation.

	Typically these are written to a http.Request.
*/
type UpdateAPIKeyParams struct {

	// Body.
	Body *models.UpdateAPIKey

	// ID.
	ID string `json:"id,omitempty"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the update api key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UpdateAPIKeyParams) WithDefaults() *UpdateAPIKeyParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the update api key params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UpdateAPIKeyParams) SetDefaults() {
	val := UpdateAPIKeyParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the update api key params
func (o *UpdateAPIKeyParams) WithTimeout(timeout time.Duration) *UpdateAPIKeyParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the update api key params
func (o *UpdateAPIKeyParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the update api key params
func (o *UpdateAPIKeyParams) WithContext(ctx context.Context) *UpdateAPIKeyParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the update api key params
func (o *UpdateAPIKeyParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the update api key params
func (o *UpdateAPIKeyParams) WithHTTPClient(client *http.Client) *UpdateAPIKeyParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the update api key params
func (o *UpdateAPIKeyParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the update api key params
func (o *UpdateAPIKeyParams) WithBody(body *models.UpdateAPIKey) *UpdateAPIKeyParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the update api key params
func (o *UpdateAPIKeyParams) SetBody(body *models.UpdateAPIKey) {
	o.Body = body
}

// WithID adds the id to the update api key params
func (o *UpdateAPIKeyParams) WithID(id string) *UpdateAPIKeyParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the update api key params
func (o *UpdateAPIKeyParams) SetID(id string) {
	o.ID = id
}

// WriteToRequest writes these params to a swagger request
func (o *UpdateAPIKeyParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if o.Body != nil {
		if err := r.SetBodyParam(o.Body); err != nil {
			return err
		}
	}

	// path param id
	if err := r.SetPathParam("id", o.ID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
