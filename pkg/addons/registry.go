package addons

type Plugin interface {
	Run(config map[string]string) error
}

var Registry = map[string]Plugin{}
