package projects

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

// UpdateProjectReader is a Reader for the UpdateProject structure.
type UpdateProjectReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *UpdateProjectReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewUpdateProjectOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
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
		return nil, runtime.NewAPIError("[PATCH /projects/{id_or_slug}] update-project", response, response.Code())
	}
}

// NewUpdateProjectOK creates a UpdateProjectOK with default headers values
func NewUpdateProjectOK() *UpdateProjectOK {
	return &UpdateProjectOK{}
}

/*
UpdateProjectOK describes a response with status code 200, with default header values.

Success
*/
type UpdateProjectOK struct {
	Payload *UpdateProjectOKBody
}

// IsSuccess returns true when this update project o k response has a 2xx status code
func (o *UpdateProjectOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this update project o k response has a 3xx status code
func (o *UpdateProjectOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update project o k response has a 4xx status code
func (o *UpdateProjectOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this update project o k response has a 5xx status code
func (o *UpdateProjectOK) IsServerError() bool {
	return false
}

// IsCode returns true when this update project o k response a status code equal to that given
func (o *UpdateProjectOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the update project o k response
func (o *UpdateProjectOK) Code() int {
	return 200
}

func (o *UpdateProjectOK) Error() string {
	return fmt.Sprintf("[PATCH /projects/{id_or_slug}][%d] updateProjectOK  %+v", 200, o.Payload)
}

func (o *UpdateProjectOK) String() string {
	return fmt.Sprintf("[PATCH /projects/{id_or_slug}][%d] updateProjectOK  %+v", 200, o.Payload)
}

func (o *UpdateProjectOK) GetPayload() *UpdateProjectOKBody {
	return o.Payload
}

func (o *UpdateProjectOK) GetData() []renderer.ResponseData {
	return []renderer.ResponseData{o.Payload.Data}
}

func (o *UpdateProjectOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	o.Payload = new(UpdateProjectOKBody)

	// response payload
	if err := consumer.Consume(response.Body(), o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewUpdateProjectForbidden creates a UpdateProjectForbidden with default headers values
func NewUpdateProjectForbidden() *UpdateProjectForbidden {
	return &UpdateProjectForbidden{}
}

/*
UpdateProjectForbidden describes a response with status code 403, with default header values.

Forbidden
*/
type UpdateProjectForbidden struct {
	Payload *models.ErrorObject
}

// IsSuccess returns true when this update project forbidden response has a 2xx status code
func (o *UpdateProjectForbidden) IsSuccess() bool {
	return false
}

// IsRedirect returns true when this update project forbidden response has a 3xx status code
func (o *UpdateProjectForbidden) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update project forbidden response has a 4xx status code
func (o *UpdateProjectForbidden) IsClientError() bool {
	return true
}

// IsServerError returns true when this update project forbidden response has a 5xx status code
func (o *UpdateProjectForbidden) IsServerError() bool {
	return false
}

// IsCode returns true when this update project forbidden response a status code equal to that given
func (o *UpdateProjectForbidden) IsCode(code int) bool {
	return code == 403
}

// Code gets the status code for the update project forbidden response
func (o *UpdateProjectForbidden) Code() int {
	return 403
}

func (o *UpdateProjectForbidden) Error() string {
	return fmt.Sprintf("[PATCH /projects/{id_or_slug}][%d] updateProjectForbidden  %+v", 403, o.Payload)
}

func (o *UpdateProjectForbidden) String() string {
	return fmt.Sprintf("[PATCH /projects/{id_or_slug}][%d] updateProjectForbidden  %+v", 403, o.Payload)
}

func (o *UpdateProjectForbidden) GetPayload() *models.ErrorObject {
	return o.Payload
}

/*
UpdateProjectBody update project body
swagger:model UpdateProjectBody
*/
type UpdateProjectBody struct {

	// data
	// Required: true
	Data *UpdateProjectParamsBodyData `json:"data"`
}

// Validate validates this update project body
func (o *UpdateProjectBody) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateData(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *UpdateProjectBody) validateData(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data", "body", o.Data); err != nil {
		return err
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

// ContextValidate validate this update project body based on the context it is used
func (o *UpdateProjectBody) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateData(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *UpdateProjectBody) contextValidateData(ctx context.Context, formats strfmt.Registry) error {

	if o.Data != nil {

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
func (o *UpdateProjectBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *UpdateProjectBody) UnmarshalBinary(b []byte) error {
	var res UpdateProjectBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
UpdateProjectOKBody update project o k body
swagger:model UpdateProjectOKBody
*/
type UpdateProjectOKBody struct {

	// data
	Data *models.Project `json:"data,omitempty"`
}

// Validate validates this update project o k body
func (o *UpdateProjectOKBody) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateData(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *UpdateProjectOKBody) validateData(formats strfmt.Registry) error {
	if swag.IsZero(o.Data) { // not required
		return nil
	}

	if o.Data != nil {
		if err := o.Data.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("updateProjectOK" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("updateProjectOK" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// ContextValidate validate this update project o k body based on the context it is used
func (o *UpdateProjectOKBody) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateData(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *UpdateProjectOKBody) contextValidateData(ctx context.Context, formats strfmt.Registry) error {

	if o.Data != nil {

		if swag.IsZero(o.Data) { // not required
			return nil
		}

		if err := o.Data.ContextValidate(ctx, formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("updateProjectOK" + "." + "data")
			} else if ce, ok := err.(*errors.CompositeError); ok {
				return ce.ValidateName("updateProjectOK" + "." + "data")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (o *UpdateProjectOKBody) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *UpdateProjectOKBody) UnmarshalBinary(b []byte) error {
	var res UpdateProjectOKBody
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
UpdateProjectParamsBodyData update project params body data
swagger:model UpdateProjectParamsBodyData
*/
type UpdateProjectParamsBodyData struct {
	Attributes *UpdateProjectParamsBodyDataAttributes `json:"attributes,omitempty"`
	ID         string                                 `json:"id"`
	Type       string                                 `json:"type"`
}

// Validate validates this update project params body data
func (o *UpdateProjectParamsBodyData) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateAttributes(formats); err != nil {
		res = append(res, err)
	}

	if err := o.validateID(formats); err != nil {
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

func (o *UpdateProjectParamsBodyData) validateAttributes(formats strfmt.Registry) error {
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

func (o *UpdateProjectParamsBodyData) validateID(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"id", "body", o.ID); err != nil {
		return err
	}

	return nil
}

var updateProjectParamsBodyDataTypeTypePropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["projects"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		updateProjectParamsBodyDataTypeTypePropEnum = append(updateProjectParamsBodyDataTypeTypePropEnum, v)
	}
}

const (

	// UpdateProjectParamsBodyDataTypeProjects captures enum value "projects"
	UpdateProjectParamsBodyDataTypeProjects string = "projects"
)

// prop value enum
func (o *UpdateProjectParamsBodyData) validateTypeEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, updateProjectParamsBodyDataTypeTypePropEnum, true); err != nil {
		return err
	}
	return nil
}

func (o *UpdateProjectParamsBodyData) validateType(formats strfmt.Registry) error {

	if err := validate.Required("body"+"."+"data"+"."+"type", "body", o.Type); err != nil {
		return err
	}

	// value enum
	if err := o.validateTypeEnum("body"+"."+"data"+"."+"type", "body", o.Type); err != nil {
		return err
	}

	return nil
}

// ContextValidate validate this update project params body data based on the context it is used
func (o *UpdateProjectParamsBodyData) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	var res []error

	if err := o.contextValidateAttributes(ctx, formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (o *UpdateProjectParamsBodyData) contextValidateAttributes(ctx context.Context, formats strfmt.Registry) error {

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
func (o *UpdateProjectParamsBodyData) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *UpdateProjectParamsBodyData) UnmarshalBinary(b []byte) error {
	var res UpdateProjectParamsBodyData
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}

/*
UpdateProjectParamsBodyDataAttributes update project params body data attributes
swagger:model UpdateProjectParamsBodyDataAttributes
*/
type UpdateProjectParamsBodyDataAttributes struct {

	// bandwidth alert
	BandwidthAlert bool `json:"bandwidth_alert,omitempty"`

	// description
	Description string `json:"description,omitempty"`

	// environment
	// Enum: [Development Staging Production]
	Environment string `json:"environment,omitempty"`

	// name
	Name string `json:"name,omitempty"`

	Tags []string `json:"tags,omitempty"`
}

// Validate validates this update project params body data attributes
func (o *UpdateProjectParamsBodyDataAttributes) Validate(formats strfmt.Registry) error {
	var res []error

	if err := o.validateEnvironment(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

var updateProjectParamsBodyDataAttributesTypeEnvironmentPropEnum []interface{}

func init() {
	var res []string
	if err := json.Unmarshal([]byte(`["Development","Staging","Production"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		updateProjectParamsBodyDataAttributesTypeEnvironmentPropEnum = append(updateProjectParamsBodyDataAttributesTypeEnvironmentPropEnum, v)
	}
}

const (

	// UpdateProjectParamsBodyDataAttributesEnvironmentDevelopment captures enum value "Development"
	UpdateProjectParamsBodyDataAttributesEnvironmentDevelopment string = "Development"

	// UpdateProjectParamsBodyDataAttributesEnvironmentStaging captures enum value "Staging"
	UpdateProjectParamsBodyDataAttributesEnvironmentStaging string = "Staging"

	// UpdateProjectParamsBodyDataAttributesEnvironmentProduction captures enum value "Production"
	UpdateProjectParamsBodyDataAttributesEnvironmentProduction string = "Production"
)

// prop value enum
func (o *UpdateProjectParamsBodyDataAttributes) validateEnvironmentEnum(path, location string, value string) error {
	if err := validate.EnumCase(path, location, value, updateProjectParamsBodyDataAttributesTypeEnvironmentPropEnum, true); err != nil {
		return err
	}
	return nil
}

func (o *UpdateProjectParamsBodyDataAttributes) validateEnvironment(formats strfmt.Registry) error {
	if swag.IsZero(o.Environment) { // not required
		return nil
	}

	// value enum
	if err := o.validateEnvironmentEnum("body"+"."+"data"+"."+"attributes"+"."+"environment", "body", o.Environment); err != nil {
		return err
	}

	return nil
}

// ContextValidate validates this update project params body data attributes based on context it is used
func (o *UpdateProjectParamsBodyDataAttributes) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (o *UpdateProjectParamsBodyDataAttributes) MarshalBinary() ([]byte, error) {
	if o == nil {
		return nil, nil
	}
	return swag.WriteJSON(o)
}

// UnmarshalBinary interface implementation
func (o *UpdateProjectParamsBodyDataAttributes) UnmarshalBinary(b []byte) error {
	var res UpdateProjectParamsBodyDataAttributes
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*o = res
	return nil
}
