package model

type GitCredentials struct {
	User      string `json:"user,omitempty"`
	Token     string `json:"token,omitempty"`
	RemoteURI string `json:"remoteURI,omitempty"`
}

type KeptnConfig struct {
	Metadata KeptnConfigMeta `yaml:"metadata,omitempty"`
	Services []KeptnService  `yaml:"services,omitempty"`
}

type KeptnConfigMeta struct {
	InitBranch string `yaml:"initbranch,omitempty"`
}

type KeptnService struct {
	Name              string `yaml:"name,omitempty"`
	DeploymentTrigger string `yaml:"triggerevent"`
	Stage             string `yaml:"stage"`
}

type KeptnTriggerEvent struct {
	ContentType string         `json:"contenttype,omitempty"`
	Data        KeptnEventData `json:"data,omitempty"`
	Source      string         `json:"source,omitempty"`
	SpecVersion string         `json:"specversion,omitempty"`
	Type        string         `json:"type,omitempty"`
}

type KeptnEventData struct {
	Project             string                  `json:"project,omitempty"`
	Service             string                  `json:"service,omitempty"`
	Stage               string                  `json:"stage,omitempty"`
	Image               string                  `json:"image,omitempty"`
	Labels              map[string]string       `json:"labels,omitempty"`
	ConfigurationChange ConfigurationChangeData `json:"configurationChange,omitempty"`
}

type ConfigurationChangeData struct {
	Values map[string]string `json:"values,omitempty"`
}

type DeploymentConfig struct {
	Metadata DeploymentConfigMeta `yaml:"metadata,omitempty"`
}

type DeploymentConfigMeta struct {
	ImageVersion     string `yaml:"imageVersion,omitempty"`
	SourceCommitHash string `yaml:"gitCommit,omitempty"`
	Author           string `yaml:"author,omitempty"`
}
