package service

type ServiceType string

const (
	SPAGHETTI         ServiceType = "SPAGHETTI"
	CATEGORIZER       ServiceType = "CATEGORIZER"
	STATIC            ServiceType = "STATIC"
	GATEWAY           ServiceType = "GATEWAY"
	DEVELOPER_GATEWAY ServiceType = "DEVELOPER_GATEWAY"
	READER            ServiceType = "READER"
	WRITER            ServiceType = "WRITER"
	BUNDLE            ServiceType = "BUNDLE"
)

// Returns the string represantion of the service type
func (s ServiceType) ToString() string {
	return string(s)
}

func service_types() []ServiceType {
	return []ServiceType{
		SPAGHETTI,
		CATEGORIZER,
		STATIC,
		GATEWAY,
		DEVELOPER_GATEWAY,
		READER,
		WRITER,
		BUNDLE,
	}
}
