package agx

#Agent: "codex" | "claude" | "gemini"

#ProfileView: {
	name!:       string & !=""
	base_url!:   string & !=""
	api_key!:    string & !=""
	created_at?: string & !=""
	updated_at?: string & !=""
}

#BindingView: {
	agent!:           #Agent
	relay!:           string & !=""
	status!:          string & !=""
	config_path!:     string & !=""
	last_applied_at?: string & !=""
	last_backup_id?:  string & !=""
}

#BackupView: {
	id!:             string & !=""
	applied_relay?:  string & !=""
	config_path?:    string & !=""
	backup_path?:    string & !=""
	restore_mode?:   string & !=""
	created_at?:     string & !=""
}

#ListView: {
	relays!: [...{
		name!:     string & !=""
		base_url!: string & !=""
		agents!:   [...#Agent]
	}]
}

#ListAgentView: {
	agent!:         #Agent
	current_relay?: string
	relays!: [...{
		name!:     string & !=""
		base_url!: string & !=""
		current?:  bool
	}]
}

#ShowView: {
	relay!:          #ProfileView
	agents!:         [...#Agent]
	agent_bindings!: [...#BindingView]
}

#BindingChangeView: {
	agent!:         #Agent
	action!:        "bind" | "unbind"
	binding?:       #BindingView
	backup!:        #BackupView
	codex_profile?: string & !=""
}

#SetView: {
	relay!:   #ProfileView
	changes!: [...#BindingChangeView]
}

#AddView: {
	relay!: #ProfileView
}

#BackupListView: {
	agent!:   #Agent
	backups!: [...#BackupView]
}

#RestoreView: {
	agent!:       #Agent
	config_path!: string & !=""
	backup!:      #BackupView
}

#RemoveView: {
	relay!: #ProfileView
}

#DoctorIssue: {
	severity!: string & !=""
	code!:     string & !=""
	message!:  string & !=""
}

#OperationRecord: {
	id!:         string & !=""
	command!:    string & !=""
	agent!:      #Agent
	relay?:      string & !=""
	backup_id?:  string & !=""
	config_path?: string & !=""
	backup_path?: string & !=""
	stage!:      string & !=""
	started_at!: string & !=""
	updated_at!: string & !=""
}

#DoctorView: {
	ok!:        bool
	operation?: #OperationRecord
	issues!:    [...#DoctorIssue]
}

#StateBinding: close({
	"source-profile"?: string & !=""
	status?:           "applied"
	"config-path"?:    string & !=""
	"last-applied-at"?: string & !=""
	"last-backup-id"?:  string & !=""
	backups?:        [...#StateBackup]
})

#StateBackup: close({
	id!:              string & !=""
	"applied-profile"?: string & !=""
	"config-path"?:     string & !=""
	"backup-path"?:     string & !=""
	"restore-mode"?:    "restore_file" | "remove_created_file"
	"created-at"?:      string & !=""
})

#CodexProfileState: close({
	status?:           "applied"
	"config-path"?:    string & !=""
	"last-applied-at"?: string & !=""
	"last-backup-id"?:  string & !=""
})

#StateFile: close({
	codex?: #StateBinding
	"codex-profiles"?: [string]: #CodexProfileState
	claude?: #StateBinding
	gemini?: #StateBinding
	"updated-at"?: string & !=""
})
