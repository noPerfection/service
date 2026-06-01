package flag

import "github.com/noPerfection/protocol/handler/config"

func ManagerName(url string) string {
	fileName := config.UrlToFileName(url)
	return "manager." + fileName
}
