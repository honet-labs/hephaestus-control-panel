package domain

import (
	"time"
)

// ==================== AUTH & USER DOMAIN ====================

type User struct {
	ID                  int       `json:"id"`
	Username            string    `json:"username"`
	PasswordHash        string    `json:"-"`
	Role                string    `json:"role"`
	ForcePasswordChange bool      `json:"forcePasswordChange"`
	CreatedAt           time.Time `json:"createdAt"`
}

type UserSession struct {
	ID        int       `json:"id"`
	UserID    int       `json:"userId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type SystemRole struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ActivityLog struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Module    string    `json:"module"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	Status    string    `json:"status"`
	UserID    *int      `json:"userId"`
	Username  *string   `json:"username,omitempty"`
}

// ==================== CONFIG & MONITORING DOMAIN ====================

type AppConfig struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type GrafanaConfig struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Host          string    `json:"host"`
	Token         string    `json:"token"`
	DatasourceUID string    `json:"datasourceUid"`
	IsActive      bool      `json:"isActive"`
	CreatedAt     time.Time `json:"createdAt"`
}

type PrometheusConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Mode        string    `json:"mode"`
	Path        string    `json:"path"`
	ReloadURL   string    `json:"reloadUrl"`
	SSHHost     *string   `json:"sshHost,omitempty"`
	SSHPort     *int      `json:"sshPort,omitempty"`
	SSHUser     *string   `json:"sshUser,omitempty"`
	SSHAuth     *string   `json:"sshAuth,omitempty"`
	SSHPassword *string   `json:"sshPassword,omitempty"`
	SSHKey      *string   `json:"sshKey,omitempty"`
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
}

type MonitoringView struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Interval    int       `json:"interval"`
	Mode        string    `json:"mode"`
	Panels      any       `json:"panels"`
	CreatedAt   time.Time `json:"createdAt"`
}

type QueryPanel struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	DatasourceType string    `json:"datasourceType"`
	DatasourceUID  string    `json:"datasourceUid"`
	TimeRangeFrom  string    `json:"timeRangeFrom"`
	TimeRangeTo    string    `json:"timeRangeTo"`
	Step           string    `json:"step"`
	Columns        any       `json:"columns"`
	CreatedAt      time.Time `json:"createdAt"`
}

type UptimeKumaConfig struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
}

type DataPrepperConfig struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Mode         string    `json:"mode"`
	PipelinesDir string    `json:"pipelinesDir"`
	ReloadURL    *string   `json:"reloadUrl,omitempty"`
	SSHHost      *string   `json:"sshHost,omitempty"`
	SSHPort      *int      `json:"sshPort,omitempty"`
	SSHUser      *string   `json:"sshUser,omitempty"`
	SSHAuth      *string   `json:"sshAuth,omitempty"`
	SSHPassword  *string   `json:"sshPassword,omitempty"`
	SSHKey       *string   `json:"sshKey,omitempty"`
	IsActive     bool      `json:"isActive"`
	CreatedAt    time.Time `json:"createdAt"`
}

type OpenSearchConfig struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	UseSSL    bool      `json:"useSsl"`
	VerifySSL bool      `json:"verifySsl"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
}

// ==================== SNMP DOMAIN ====================

type ImportedMib struct {
	Name       string    `json:"name"`
	NodeCount  int       `json:"nodeCount"`
	ImportedAt time.Time `json:"importedAt"`
}

type OidRegistry struct {
	OID         string    `json:"oid"`
	Name        string    `json:"name"`
	MibName     string    `json:"mibName"`
	Syntax      *string   `json:"syntax,omitempty"`
	Access      *string   `json:"access,omitempty"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type SnmpQueryResult struct {
	OID   string `json:"oid"`
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  string `json:"type"`
}

// ==================== REMOTE HOST DOMAIN ====================

type RemoteHostConfig struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Username  string    `json:"username"`
	AuthType  string    `json:"authType"`
	Password  *string   `json:"password,omitempty"`
	SSHKey    *string   `json:"sshKey,omitempty"`
	GroupName string    `json:"groupName"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
}

type SftpFileEntry struct {
	Name    string    `json:"name"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
}

type WsTerminalMessage struct {
	Type         string `json:"type"`
	Data         string `json:"data,omitempty"`
	Cols         int    `json:"cols,omitempty"`
	Rows         int    `json:"rows,omitempty"`
	Token        string `json:"token,omitempty"`
	HostID       string `json:"hostId,omitempty"`
	HostConfigID string `json:"hostConfigId,omitempty"`
	Message      string `json:"message,omitempty"`
}

// ==================== TOPOLOGY DOMAIN ====================

type TopologySheet struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type TopologyDevice struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	IPAddress  string                 `json:"ipAddress"`
	DeviceType string                 `json:"deviceType"`
	Status     string                 `json:"status"`
	Sources    []string               `json:"sources"`
	Labels     map[string]interface{} `json:"labels"`
	Interfaces []NetworkInterface     `json:"interfaces"`
	SheetID    *int                   `json:"sheetId,omitempty"`
	X          *float64               `json:"x,omitempty"`
	Y          *float64               `json:"y,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
}

type NetworkInterface struct {
	Name     string `json:"name"`
	IP       string `json:"ip"`
	MAC      string `json:"mac"`
	Speed    int64  `json:"speed"`
	SpeedStr string `json:"speedStr"`
	Status   string `json:"status"`
}

type TopologyEdge struct {
	ID          int       `json:"id"`
	SourceID    string    `json:"sourceId"`
	TargetID    string    `json:"targetId"`
	Label       *string   `json:"label,omitempty"`
	SourceLabel *string   `json:"sourceLabel,omitempty"`
	TargetLabel *string   `json:"targetLabel,omitempty"`
	EdgeType    string    `json:"edgeType"`
	SheetID     *int      `json:"sheetId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type DevicePingResult struct {
	DeviceID  string    `json:"deviceId"`
	IP        string    `json:"ip"`
	Reachable bool      `json:"reachable"`
	LatencyMS *float64  `json:"latencyMs,omitempty"`
	CheckedAt time.Time `json:"checkedAt"`
}

// ==================== DATABASE BACKUP DOMAIN ====================

type BackupDbConfig struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	DBType       string    `json:"dbType"` // postgresql, mysql, mariadb, sqlserver
	Host         string    `json:"host"`
	Port         int       `json:"port"`
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	DatabaseName string    `json:"databaseName"`
	SSHHost      *string   `json:"sshHost,omitempty"`
	SSHPort      *int      `json:"sshPort,omitempty"`
	SSHUser      *string   `json:"sshUser,omitempty"`
	SSHAuth      *string   `json:"sshAuth,omitempty"`
	SSHPassword  *string   `json:"sshPassword,omitempty"`
	SSHKey       *string   `json:"sshKey,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type BackupDestination struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	DestType  string                 `json:"destType"` // local, r2, s3, gdrive, nas
	Config    map[string]interface{} `json:"config"`
	IsActive  bool                   `json:"isActive"`
	CreatedAt time.Time              `json:"createdAt"`
}

type BackupHistoryEntry struct {
	ID            string     `json:"id"`
	DBConfigID    *string    `json:"dbConfigId,omitempty"`
	DestinationID *string    `json:"destinationId,omitempty"`
	DBName        string     `json:"dbName"`
	DBType        string     `json:"dbType"`
	DestType      string     `json:"destType"`
	Filename      string     `json:"filename"`
	FileSize      int64      `json:"fileSize"`
	Status        string     `json:"status"` // running, success, failed, cancelled
	ErrorMessage  *string    `json:"errorMessage,omitempty"`
	StartedAt     time.Time  `json:"startedAt"`
	CompletedAt   *time.Time `json:"completedAt,omitempty"`
}

type BackupSchedule struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	DBConfigID     string     `json:"dbConfigId"`
	DestinationID  string     `json:"destinationId"`
	CronExpression string     `json:"cronExpression"`
	IsActive       bool       `json:"isActive"`
	LastRun        *time.Time `json:"lastRun,omitempty"`
	NextRun        *time.Time `json:"nextRun,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// ==================== BACKGROUND JOB & QUEUE DOMAIN ====================

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

type Job struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"` // backup, icmp_ping, topology_scan, snmp_sync, etc.
	Status      JobStatus              `json:"status"`
	Payload     map[string]interface{} `json:"payload"`
	Progress    int                    `json:"progress"` // 0-100%
	Message     string                 `json:"message"`
	Error       string                 `json:"error,omitempty"`
	Retries     int                    `json:"retries"`
	MaxRetries  int                    `json:"maxRetries"`
	CreatedAt   time.Time              `json:"createdAt"`
	StartedAt   *time.Time             `json:"startedAt,omitempty"`
	CompletedAt *time.Time             `json:"completedAt,omitempty"`
}

// ==================== VPS & SYSTEM TELEMETRY DOMAIN ====================

type SystemStats struct {
	AppVersion     string    `json:"appVersion"`
	UptimeSeconds  int64     `json:"uptimeSeconds"`
	GoroutineCount int       `json:"goroutineCount"`
	MemoryAllocMB  float64   `json:"memoryAllocMb"`
	MemoryTotalMB  float64   `json:"memoryTotalMb"`
	CPUUsagePct    float64   `json:"cpuUsagePct"`
	DatabaseStatus string    `json:"databaseStatus"`
	Timestamp      time.Time `json:"timestamp"`
}

type VpsMetrics struct {
	Hostname   string             `json:"hostname"`
	OS         string             `json:"os"`
	Kernel     string             `json:"kernel"`
	Arch       string             `json:"arch"`
	Uptime     string             `json:"uptime"`
	LoadAvg    []float64          `json:"loadAvg"`
	CPU        CpuInfo            `json:"cpu"`
	Memory     MemoryInfo         `json:"memory"`
	Disks      []DiskInfo         `json:"disks"`
	Processes  []ProcessInfo      `json:"processes"`
	Services   []ServiceInfo      `json:"services"`
	Interfaces []NetworkInterface `json:"interfaces"`
}

type CpuInfo struct {
	Model string  `json:"model"`
	Cores int     `json:"cores"`
	Usage float64 `json:"usage"`
}

type MemoryInfo struct {
	Total     int64   `json:"total"`
	Used      int64   `json:"used"`
	Free      int64   `json:"free"`
	Available int64   `json:"available"`
	Percent   float64 `json:"percent"`
}

type DiskInfo struct {
	Filesystem string  `json:"filesystem"`
	Mount      string  `json:"mount"`
	Total      string  `json:"total"`
	Used       string  `json:"used"`
	Available  string  `json:"available"`
	Percent    float64 `json:"percent"`
}

type ProcessInfo struct {
	PID     int     `json:"pid"`
	User    string  `json:"user"`
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"`
	VSZ     int64   `json:"vsz"`
	RSS     int64   `json:"rss"`
	Command string  `json:"command"`
}

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ActiveState string `json:"activeState"`
	SubState    string `json:"subState"`
}

// ==================== NOTIFICATION DOMAIN ====================

type NotificationPayload struct {
	Title     string                 `json:"title"`
	Message   string                 `json:"message"`
	Level     string                 `json:"level"` // info, warning, error, success
	Module    string                 `json:"module"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
