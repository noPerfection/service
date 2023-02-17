package service

type ServiceType string

const (
	SPAGHETTI         ServiceType = "SPAGHETTI"
	CATEGORIZER       ServiceType = "CATEGORIZER"
	STATIC            ServiceType = "STATIC"
	GATEWAY           ServiceType = "GATEWAY"
	DEVELOPER_GATEWAY ServiceType = "DEVELOPER_GATEWAY"
	PUBLISHER         ServiceType = "PUBLISHER"
	READER            ServiceType = "READER"
	WRITER            ServiceType = "WRITER"
	BUNDLE            ServiceType = "BUNDLE"
	LOG               ServiceType = "LOG"
)

func service_types() []ServiceType {
	return []ServiceType{
		SPAGHETTI,
		CATEGORIZER,
		GATEWAY,
		DEVELOPER_GATEWAY,
		PUBLISHER,
		READER,
		WRITER,
		BUNDLE,
		LOG,
	}
}
