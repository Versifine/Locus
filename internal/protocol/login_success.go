package protocol

import "io"

type LoginSuccess struct {
	UUID             UUID
	Username         string
	PropertiesLength int32
	Properties       []Property
}

type Property struct {
	Name      string
	Value     string
	Signature *string
}

func ParseLoginSuccess(r io.Reader) (*LoginSuccess, error) {
	uuid, err := ReadUUID(r)
	if err != nil {
		return nil, err
	}
	username, err := ReadString(r)
	if err != nil {
		return nil, err
	}
	propertiesLength, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	properties := make([]Property, propertiesLength)
	for i := int32(0); i < propertiesLength; i++ {
		prop, err := ReadProperty(r)
		if err != nil {
			return nil, err
		}
		properties[i] = prop
	}
	return &LoginSuccess{
		UUID:             uuid,
		Username:         username,
		PropertiesLength: propertiesLength,
		Properties:       properties,
	}, nil
}

func ReadProperty(r io.Reader) (Property, error) {
	name, err := ReadString(r)
	if err != nil {
		return Property{}, err
	}
	value, err := ReadString(r)
	if err != nil {
		return Property{}, err
	}
	hasSignature, err := ReadBool(r)
	if err != nil {
		return Property{}, err
	}
	var signature *string
	if hasSignature {
		sig, err := ReadString(r)
		if err != nil {
			return Property{}, err
		}
		signature = &sig
	}
	return Property{
		Name:      name,
		Value:     value,
		Signature: signature,
	}, nil
}
