package deploys

// UpdateInput is PATCH /deploys/:id. Nil fields are left unchanged.
type UpdateInput struct {
	Name           *string            `json:"name"`
	DefaultBranch  *string            `json:"default_branch"`
	ServerID       *string            `json:"server_id"`
	Language       *string            `json:"language"`
	InstallScript  *string            `json:"install_script"`
	RootDirectory  *string            `json:"root_directory"`
	ClonePath      *string            `json:"clone_path"`
	Port           *int               `json:"port"`
	ProcessManager *string            `json:"process_manager"`
	Env            *map[string]string `json:"env"`
	Apps           *[]SpecApp         `json:"apps"`
	WriteSpec      *bool              `json:"write_spec"`
	AutoRollback   *bool              `json:"auto_rollback"`
}

func applyUpdate(p Project, in UpdateInput) Project {
	if in.Name != nil {
		p.Name = *in.Name
	}
	if in.DefaultBranch != nil {
		p.DefaultBranch = *in.DefaultBranch
	}
	if in.ServerID != nil {
		p.ServerID = *in.ServerID
	}
	if in.Language != nil {
		p.Language = *in.Language
	}
	if in.InstallScript != nil {
		p.InstallScript = *in.InstallScript
	}
	if in.RootDirectory != nil {
		p.RootDirectory = *in.RootDirectory
	}
	if in.ClonePath != nil {
		p.ClonePath = *in.ClonePath
	}
	if in.Port != nil {
		p.Port = *in.Port
	}
	if in.ProcessManager != nil {
		p.ProcessManager = *in.ProcessManager
	}
	if in.Env != nil {
		p.Env = *in.Env
	}
	if in.Apps != nil {
		p.Apps = *in.Apps
	}
	if in.WriteSpec != nil {
		p.WriteSpec = *in.WriteSpec
	}
	if in.AutoRollback != nil {
		p.AutoRollback = *in.AutoRollback
	}
	return p
}
