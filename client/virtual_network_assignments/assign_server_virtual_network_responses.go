package virtual_network_assignments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"

	apierrors "github.com/latitudesh/lsh/internal/api/errors"
	"github.com/latitudesh/lsh/internal/renderer"
	"github.com/latitudesh/lsh/models"
)

// AssignServerVirtualNetworkReader is a Reader for the AssignServerVirtualNetwork structure.
type AssignServerVirtualNetworkReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *AssignServerVirtualNetworkReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 201:
		result := NewAssignServerVirtualNetworkCreated()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	case 400:
		result := apierrors.NewBadRequest()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 401:
		result := apierrors.NewUnauthorized()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 403:
		result := apierrors.NewForbidden()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 404:
		result := apierrors.NewNotFound()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	case 422:
		result := apierrors.NewUnprocessableEntity()
		if err := result.ReadResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return nil, result
	default:
		return nil, runtime.NewAPIError("[POST /virtual_networks/assignments] assign-server-virtual-network", response, response.Code())
	}
}

// NewAssignServerVirtualNetworkCreated creates a AssignServerVirtualNetworkCreated with default headers values
func NewAssignServerVirtualNetworkCreated() *AssignServerVirtualNetworkCreated {
	return &AssignServerVirtualNetworkCreated{}
}

/*
AssignServerVirtualNetworkCreated describes a response with status code 201, with default header values.

Created
*/
type AssignServerVirtualNetworkCreated struct {
	Payload *models.VirtualNetworkAssignmentPayload
}

// IsSuccess returns true when this assign server virtual network created response has a 2xx status code
func (o *AssignServerVirtualNetworkCreated) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this assign server virtual network created response has a 3xx status code
func (o *AssignServerVirtualNetworkCreated) IsRedirect() bool {
	return false
}

// IsClientError returns true when this assign server virtual network created response has a 4xx status code
func (o *AssignServerVirtualNetworkCreated) IsClientError() bool {
	return false
}

// IsServerError returns true when this assign server virtual network created response has a 5xx status code
func (o *AssignServerVirtualNetworkCreated) IsServerError() bool {
	return false
}

// IsCode returns true when this assign server virtual network created response a status code equal to that given
func (o *AssignServerVirtualNetworkCreated) IsCode(code int) bool {
	return code == 201
}

// Code gets the status code for the assign server virtual network created response
func (o *AssignServerVirtualNetworkCreated) Code() int {
	return 201
}

func (o *AssignServerVirtualNetworkCreated) Error() string {
	return fmt.Sprintf("[POST /virtual_networks/assignments][%d] assignServerVirtualNetworkCreated  %+v", 201, o.Payload)
}

func (o *AssignServerVirtualNetworkCreated) String() string {
	return fmt.Sprintf("[POST /virtual_networks/assignments][%d] assignServerVirtualNetworkCreated  %+v", 201, o.Payload)
}

func (o *AssignServerVirtualNetworkCreated) GetPayload() *models.VirtualNetworkAssignmentPayload {
	return o.Payload
}

func (o *AssignServerVirtualNetworkCreated) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{o.Payload.Data}
}

func (o *AssignServerVirtualNetworkCreated) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(models.VirtualNetworkAssignmentPayload)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

/*
AssignServerVirtualNetworkBody assign server virtual network body
swagger:model AssignServerVirtualNetworkBody
*/
type AssignServerVirtualNetworkBody struct {

	// data
	Data *AssignServerVirtualNetworkParamsBodyData `json:"data,omitempty"`
}

// Validate validates this assign server virtual network body
func (o *AssignServerVirtualNetworkBody) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateData(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *AssignServerVirtualNetworkBody) validateData(formats strfmt.Registry) error {
	if swag.IsZero(o.Data) { // not required
		return nil
	}

	if o.Data != nil {
		if err := o.Data.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this assign server virtual network body based on the context it is used
func (o *AssignServerVirtualNetworkBody) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateData(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *AssignServerVirtualNetworkBody) contextValidateData(ctx context.Context, formats strfmt.Registry) error {

	if o.Data != nil {

		if swag.IsZero(o.Data) { // not required
			return nil
		}

		if err := o.Data.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (o *AssignServerVirtualNetworkBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *AssignServerVirtualNetworkBody) UnmarshalBinary(b []byte) error {
	var res AssignServerVirtualNetworkBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
AssignServerVirtualNetworkParamsBodyData assign server virtual network params body data
swagger:model AssignServerVirtualNetworkParamsBodyData
*/
type AssignServerVirtualNetworkParamsBodyData struct {

	// attributes
	Attributes *AssignServerVirtualNetworkParamsBodyDataAttributes `json:"attributes,omitempty"`

	// type
	// Required: true
	// Enum: [virtual_network_assignment]
	Type *string `json:"type"`
}

// Validate validates this assign server virtual network params body data
func (o *AssignServerVirtualNetworkParamsBodyData) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateAttributes(formats); err != nil {
		res = append(res, err)
	}

	if err := o.validateType(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *AssignServerVirtualNetworkParamsBodyData) validateAttributes(formats strfmt.Registry) error {
	if swag.IsZero(o.Attributes) { // not required
		return nil
	}

	if o.Attributes != nil {
		if err := o.Attributes.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data" + "." + "attributes")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data" + "." + "attributes")
			}
			return err
		}
	}

	return nil
}

var assignServerVirtualNetworkParamsBodyDataTypeTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["virtual_network_assignment"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		assignServerVirtualNetworkParamsBodyDataTypeTypePropEnum = append(assignServerVirtualNetworkParamsBodyDataTypeTypePropEnum, v)
	}
}

const (

	// AssignServerVirtualNetworkParamsBodyDataTypeVirtualNetworkAssignment captures enum value "virtual_network_assignment"
	AssignServerVirtualNetworkParamsBodyDataTypeVirtualNetworkAssignment string = "virtual_network_assignment"
)

// prop value enum
func (o *AssignServerVirtualNetworkParamsBodyData) validateTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, assignServerVirtualNetworkParamsBodyDataTypeTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (o *AssignServerVirtualNetworkParamsBodyData) validateType(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"type", "body", o.Type); err != nil {
		return err
	}

	// value enum
	if err := o.validateTypeEnum("body"+"."+"data"+"."+"type", "body", *o.Type); err != nil {
		return err
	}

	return nil
}

// ContextValidate validate this assign server virtual network params body data based on the context it is used
func (o *AssignServerVirtualNetworkParamsBodyData) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateAttributes(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *AssignServerVirtualNetworkParamsBodyData) contextValidateAttributes(ctx context.Context, formats strfmt.Registry) error {

	if o.Attributes != nil {

		if swag.IsZero(o.Attributes) { // not required
			return nil
		}

		if err := o.Attributes.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("body" + "." + "data" + "." + "attributes")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("body" + "." + "data" + "." + "attributes")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (o *AssignServerVirtualNetworkParamsBodyData) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *AssignServerVirtualNetworkParamsBodyData) UnmarshalBinary(b []byte) error {
	var res AssignServerVirtualNetworkParamsBodyData
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
AssignServerVirtualNetworkParamsBodyDataAttributes assign server virtual network params body data attributes
swagger:model AssignServerVirtualNetworkParamsBodyDataAttributes
*/
type AssignServerVirtualNetworkParamsBodyDataAttributes struct {

	// server id
	// Required: true
	ServerID string `json:"server_id"`

	// virtual network id
	// Required: true
	VirtualNetworkID string `json:"virtual_network_id"`
}

// Validate validates this assign server virtual network params body data attributes
func (o *AssignServerVirtualNetworkParamsBodyDataAttributes) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateServerID(formats); err != nil {
		res = append(res, err)
	}

	if err := o.validateVirtualNetworkID(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *AssignServerVirtualNetworkParamsBodyDataAttributes) validateServerID(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"attributes"+"."+"server_id", "body", o.ServerID); err != nil {
		return err
	}

	return nil
}

func (o *AssignServerVirtualNetworkParamsBodyDataAttributes) validateVirtualNetworkID(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"attributes"+"."+"virtual_network_id", "body", o.VirtualNetworkID); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this assign server virtual network params body data attributes based on context it is used
func (o *AssignServerVirtualNetworkParamsBodyDataAttributes) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (o *AssignServerVirtualNetworkParamsBodyDataAttributes) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *AssignServerVirtualNetworkParamsBodyDataAttributes) UnmarshalBinary(b []byte) error {
	var res AssignServerVirtualNetworkParamsBodyDataAttributes
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}
