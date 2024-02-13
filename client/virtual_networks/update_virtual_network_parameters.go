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

// NewUpdateVirtualNetworkParams creates a new UpdateVirtualNetworkParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewUpdateVirtualNetworkParams() *UpdateVirtualNetworkParams {
	return &UpdateVirtualNetworkParams{
		timeout: cr.DefaultTimeout,
		Body: UpdateVirtualNetworkBody{
			Data: &UpdateVirtualNetworkParamsBodyData{
				Type:       &updatevirtualNetworkType,
				Attributes: &UpdateVirtualNetworkParamsBodyDataAttributes{},
			},
		},
	}
}

// NewUpdateVirtualNetworkParamsWithTimeout creates a new UpdateVirtualNetworkParams object
// with the ability to set a timeout on a request.
func NewUpdateVirtualNetworkParamsWithTimeout(timeout time.Duration) *UpdateVirtualNetworkParams {
	return &UpdateVirtualNetworkParams{
		timeout: timeout,
	}
}

// NewUpdateVirtualNetworkParamsWithContext creates a new UpdateVirtualNetworkParams object
// with the ability to set a context for a request.
func NewUpdateVirtualNetworkParamsWithContext(ctx context.Context) *UpdateVirtualNetworkParams {
	return &UpdateVirtualNetworkParams{
		Context: ctx,
	}
}

// NewUpdateVirtualNetworkParamsWithHTTPClient creates a new UpdateVirtualNetworkParams object
// with the ability to set a custom HTTPClient for a request.
func NewUpdateVirtualNetworkParamsWithHTTPClient(client *http.Client) *UpdateVirtualNetworkParams {
	return &UpdateVirtualNetworkParams{
		HTTPClient: client,
	}
}

/*
UpdateVirtualNetworkParams contains all the parameters to send to the API endpoint

	for the update virtual network operation.

	Typically these are written to a http.Request.
*/
type UpdateVirtualNetworkParams struct {
	Body             UpdateVirtualNetworkBody
	VirtualNetworkID string `json:"id"`

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the update virtual network params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UpdateVirtualNetworkParams) WithDefaults() *UpdateVirtualNetworkParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the update virtual network params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UpdateVirtualNetworkParams) SetDefaults() {
	val := UpdateVirtualNetworkParams{}

	val.timeout = o.timeout
	val.Context = o.Context
	val.HTTPClient = o.HTTPClient
	*o = val
}

// WithTimeout adds the timeout to the update virtual network params
func (o *UpdateVirtualNetworkParams) WithTimeout(timeout time.Duration) *UpdateVirtualNetworkParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the update virtual network params
func (o *UpdateVirtualNetworkParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the update virtual network params
func (o *UpdateVirtualNetworkParams) WithContext(ctx context.Context) *UpdateVirtualNetworkParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the update virtual network params
func (o *UpdateVirtualNetworkParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the update virtual network params
func (o *UpdateVirtualNetworkParams) WithHTTPClient(client *http.Client) *UpdateVirtualNetworkParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the update virtual network params
func (o *UpdateVirtualNetworkParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the update virtual network params
func (o *UpdateVirtualNetworkParams) WithBody(body UpdateVirtualNetworkBody) *UpdateVirtualNetworkParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the update virtual network params
func (o *UpdateVirtualNetworkParams) SetBody(body UpdateVirtualNetworkBody) {
	o.Body = body
}

// WithVirtualNetworkID adds the virtualNetworkID to the update virtual network params
func (o *UpdateVirtualNetworkParams) WithVirtualNetworkID(virtualNetworkID string) *UpdateVirtualNetworkParams {
	o.SetVirtualNetworkID(virtualNetworkID)
	return o
}

// SetVirtualNetworkID adds the virtualNetworkId to the update virtual network params
func (o *UpdateVirtualNetworkParams) SetVirtualNetworkID(virtualNetworkID string) {
	o.VirtualNetworkID = virtualNetworkID
}

// WriteToRequest writes these params to a swagger request
func (o *UpdateVirtualNetworkParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	if err := r.SetBodyParam(o.Body); err != nil {
		return err
	}

	// path param virtual_network_id
	if err := r.SetPathParam("virtual_network_id", o.VirtualNetworkID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
