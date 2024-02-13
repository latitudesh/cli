package virtual_networks

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewCreateVirtualNetworkParams creates a new CreateVirtualNetworkParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewCreateVirtualNetworkParams() *CreateVirtualNetworkParams {
	return &CreateVirtualNetworkParams{
		timeout: cr.DefaultTimeout,
		Body: CreateVirtualNetworkBody{
			Data: &CreateVirtualNetworkParamsBodyData{
				Type:       &createvirtualNetworkType,
				Attributes: &CreateVirtualNetworkParamsBodyDataAttributes{},
			},
		},
	}
}

// NewCreateVirtualNetworkParamsWithTimeout creates a new CreateVirtualNetworkParams object
// with the ability to set a timeout on a request.
func NewCreateVirtualNetworkParamsWithTimeout(timeout time.Duration) *CreateVirtualNetworkParams {
	return &CreateVirtualNetworkParams{
		timeout: timeout,
	}
}

// NewCreateVirtualNetworkParamsWithContext creates a new CreateVirtualNetworkParams object
// with the ability to set a context for a request.
func NewCreateVirtualNetworkParamsWithContext(ctx context.Context) *CreateVirtualNetworkParams {
	return &CreateVirtualNetworkParams{
		Context: ctx,
	}
}

// NewCreateVirtualNetworkParamsWithHTTPClient creates a new CreateVirtualNetworkParams object
// with the ability to set a custom HTTPClient for a request.
func NewCreateVirtualNetworkParamsWithHTTPClient(client *http.Client) *CreateVirtualNetworkParams {
	return &CreateVirtualNetworkParams{
		HTTPClient: client,
	}
}

/*
CreateVirtualNetworkParams contains all the parameters to send to the API endpoint

	for the create virtual network operation.

	Typically these are written to a http.Request.
*/
type CreateVirtualNetworkParams struct {

	// Body.
	Body CreateVirtualNetworkBody

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the create virtual network params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *CreateVirtualNetworkParams) WithDefaults() *CreateVirtualNetworkParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the create virtual network params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *CreateVirtualNetworkParams) SetDefaults() {
	val := CreateVirtualNetworkParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the create virtual network params
func (o *CreateVirtualNetworkParams) WithTimeout(timeout time.Duration) *CreateVirtualNetworkParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the create virtual network params
func (o *CreateVirtualNetworkParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the create virtual network params
func (o *CreateVirtualNetworkParams) WithContext(ctx context.Context) *CreateVirtualNetworkParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the create virtual network params
func (o *CreateVirtualNetworkParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the create virtual network params
func (o *CreateVirtualNetworkParams) WithHTTPClient(client *http.Client) *CreateVirtualNetworkParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the create virtual network params
func (o *CreateVirtualNetworkParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the create virtual network params
func (o *CreateVirtualNetworkParams) WithBody(body CreateVirtualNetworkBody) *CreateVirtualNetworkParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the create virtual network params
func (o *CreateVirtualNetworkParams) SetBody(body CreateVirtualNetworkBody) {
	o.Body = body
}

// WriteToRequest writes these params to a swagger request
func (o *CreateVirtualNetworkParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if err := r.SetBodyParam(o.Body); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
