package protocol

import "io"

type Experience struct {
	ExperienceBar   float32
	Level           int32
	TotalExperience int32
}

func ParseExperience(r io.Reader) (*Experience, error) {
	experienceBar, err := ReadFloat(r)
	if err != nil {
		return nil, err
	}
	level, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	totalExperience, err := ReadVarint(r)
	if err != nil {
		return nil, err
	}
	return &Experience{
		ExperienceBar:   experienceBar,
		Level:           int32(level),
		TotalExperience: int32(totalExperience),
	}, nil
}
