package domain

import (
	"strings"
	"time"
)

// ==================== AUTH & USER DOMAIN ====================

type User struct {
	ID                  int               `json:"id"`
	Username            string            `json:"username"`
	PasswordHash        string            `json:"-"`
	Role                string            `json:"role"`
	Permissions         map[string]string `json:"permissions"`
	ForcePasswordChange bool              `json:"forcePasswordChange"`
	CreatedAt           time.Time         `json:"createdAt"`
}

// IsAdmin returns true if the user has an administrative role or is a master admin account
func (u *User) IsAdmin() bool {
	if u == nil {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(u.Username))
	if name == "admin" || name == "administrator" || name == "root" {
		return true
	}
	return IsAdminRole(u.Role)
}

// IsAdminRole checks if a role string represents an administrator role
func IsAdminRole(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == "admin" || r == "administrator" || r == "superadmin" || r == "super_admin" || r == "root" || r == "owner"
}

type UserSession struct {
	ID        int       `json:"id"`
	UserID    int       `json:"userId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type SystemRole struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Permissions map[string]string `json:"permissions"`
	IsDefault   bool              `json:"isDefault"`
	CreatedAt   time.Time         `json:"createdAt"`
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

type OpenTelemetryConfig struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Tags        []string   `json:"tags"`
	SSHHost     string     `json:"sshHost"`
	SSHPort     int        `json:"sshPort"`
	SSHUser     string     `json:"sshUser"`
	SSHAuth     string     `json:"sshAuth"`
	SSHPassword *string    `json:"sshPassword,omitempty"`
	SSHKey      *string    `json:"sshKey,omitempty"`
	ConfigPath  string     `json:"configPath"`
	ServiceName string     `json:"serviceName"`
	ReloadMode  string     `json:"reloadMode"`
	LastStatus  string     `json:"lastStatus"`
	IsActive    bool       `json:"isActive"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type OpenTelemetryConfigHistory struct {
	ID            string    `json:"id"`
	OTelConfigID  string    `json:"otelConfigId"`
	Content       string    `json:"content"`
	CreatedBy     *string   `json:"createdBy,omitempty"`
	ChangeSummary *string   `json:"changeSummary,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
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
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	Username      string    `json:"username"`
	AuthType      string    `json:"authType"`
	Password      *string   `json:"password,omitempty"`
	SSHKey        *string   `json:"sshKey,omitempty"`
	GroupName     string    `json:"groupName"`
	Tags          []string  `json:"tags"`
	UserID        *int      `json:"userId,omitempty"`
	OwnerUsername string    `json:"ownerUsername,omitempty"`
	IsOwner       bool      `json:"isOwner"`
	SharedAccess  string    `json:"sharedAccess,omitempty"` // "owner", "admin", "read", "manage"
	SharesCount   int       `json:"sharesCount"`
	CreatedAt     time.Time `json:"createdAt"`
}

type RemoteHostShare struct {
	ID               string    `json:"id"`
	HostID           string    `json:"hostId"`
	UserID           int       `json:"userId"`
	Username         string    `json:"username"`
	Permission       string    `json:"permission"` // "read" or "manage"
	SharedBy         *int      `json:"sharedBy,omitempty"`
	SharedByUsername string    `json:"sharedByUsername,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
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

type RemoteHostFirewallRule struct {
	ID          string    `json:"id"`
	HostID      string    `json:"hostId"`
	Protocol    string    `json:"protocol"`    // ALL, TCP, UDP, ICMP
	PortRange   string    `json:"portRange"`   // ALL, 22, 80,443, 8000:8500
	SourceIP    string    `json:"sourceIp"`    // 0.0.0.0/0, CIDR, IP
	Action      string    `json:"action"`      // ALLOW, DENY
	Description string    `json:"description"` // User notes/remarks
	IsActive    bool      `json:"isActive"`
	CreatedAt   time.Time `json:"createdAt"`
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
	DBConfigIDs    []string   `json:"dbConfigIds"`
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

// ==================== VAULTWARDEN / BITWARDEN DOMAIN ====================

type VaultwardenConfig struct {
	ID             string                `json:"id"`
	Name           string                `json:"name"`
	ServerURL      string                `json:"serverUrl"`
	Email          string                `json:"email"`
	MasterPassword string                `json:"masterPassword,omitempty"`
	IsActive       bool                  `json:"isActive"`
	LastSyncedAt   *time.Time            `json:"lastSyncedAt,omitempty"`
	CachedCiphers  []VaultCredentialItem `json:"cachedCiphers,omitempty"`
	CreatedAt      time.Time             `json:"createdAt"`
	UpdatedAt      time.Time             `json:"updatedAt"`
}

type VaultCredentialItem struct {
	ID           string    `json:"id"`
	FolderID     *string   `json:"folderId,omitempty"`
	FolderName   string    `json:"folderName,omitempty"`
	Name         string    `json:"name"`
	Type         int       `json:"type"` // 1 = Login, 2 = Secure Note, 3 = Card, 4 = Identity
	TypeLabel    string    `json:"typeLabel"`
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	Notes        string    `json:"notes,omitempty"`
	URIs         []string  `json:"uris,omitempty"`
	TOTP         string    `json:"totp,omitempty"`
	RevisionDate time.Time `json:"revisionDate"`
}

type VaultSyncResponse struct {
	Success      bool                  `json:"success"`
	Message      string                `json:"message"`
	TotalItems   int                   `json:"totalItems"`
	LoginsCount  int                   `json:"loginsCount"`
	NotesCount   int                   `json:"notesCount"`
	LastSyncedAt time.Time             `json:"lastSyncedAt"`
	Items        []VaultCredentialItem `json:"items"`
}

type CreateVaultCipherRequest struct {
	Type     int      `json:"type"` // 1 = Login, 2 = Secure Note
	Name     string   `json:"name" binding:"required"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	URI      string   `json:"uri"`
	Notes    string   `json:"notes"`
	FolderID *string  `json:"folderId,omitempty"`
}

// ==================== DOCKER & CONTAINER INFRASTRUCTURE DOMAIN ====================

type DockerConnection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	HostType    string    `json:"hostType"` // "local", "ssh", "tcp"
	Driver      string    `json:"driver"`   // "socket", "ssh", "tcp"
	SocketPath  string    `json:"socketPath"`
	TcpURL      string    `json:"tcpUrl,omitempty"`
	RemoteHostID *string  `json:"remoteHostId,omitempty"`
	SSHHost     *string   `json:"sshHost,omitempty"`
	SSHPort     *int      `json:"sshPort,omitempty"`
	SSHUser     *string   `json:"sshUser,omitempty"`
	SSHAuth     *string   `json:"sshAuth,omitempty"`
	SSHPassword *string   `json:"sshPassword,omitempty"`
	SSHKey      *string   `json:"sshKey,omitempty"`
	IsActive    bool      `json:"isActive"`
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type DockerContainerPort struct {
	IP          string `json:"ip,omitempty"`
	PrivatePort int    `json:"privatePort"`
	PublicPort  int    `json:"publicPort,omitempty"`
	Type        string `json:"type"` // tcp, udp
}

type DockerContainer struct {
	ID      string                `json:"id"`
	Names   []string              `json:"names"`
	Name    string                `json:"name"`
	Image   string                `json:"image"`
	ImageID string                `json:"imageId"`
	Command string                `json:"command"`
	Created int64                 `json:"created"`
	State   string                `json:"state"` // running, exited, paused, restarting
	Status  string                `json:"status"`
	Ports     []DockerContainerPort `json:"ports"`
	Networks  []string              `json:"networks,omitempty"`
	IPAddress string                `json:"ipAddress,omitempty"`
	Labels    map[string]string     `json:"labels,omitempty"`
}

type DockerContainerStats struct {
	ContainerID   string  `json:"containerId"`
	Name          string  `json:"name,omitempty"`
	CPUPercent    float64 `json:"cpuPercent"`
	MemoryUsageMB float64 `json:"memoryUsageMb"`
	MemoryLimitMB float64 `json:"memoryLimitMb"`
	MemoryPercent float64 `json:"memoryPercent"`
	NetworkRxMB   float64 `json:"networkRxMb"`
	NetworkTxMB   float64 `json:"networkTxMb"`

	// Frontend compatibility fields
	MemUsage   int64   `json:"memUsage"`
	MemLimit   int64   `json:"memLimit"`
	MemPercent float64 `json:"memPercent"`
	NetRx      int64   `json:"netRx"`
	NetTx      int64   `json:"netTx"`
	BlockRead  int64   `json:"blockRead"`
	BlockWrite int64   `json:"blockWrite"`
	Pids       int     `json:"pids"`
}

type DockerImage struct {
	ID          string   `json:"id"`
	RepoTags    []string `json:"repoTags"`
	Size        int64    `json:"size"`
	SizeMB      float64  `json:"sizeMb"`
	VirtualSize int64    `json:"virtualSize,omitempty"`
	Created     int64    `json:"created"`
	CreatedStr  string   `json:"createdStr,omitempty"`
	Containers  int      `json:"containers"`
}

type DockerNetwork struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Driver          string            `json:"driver"`
	Scope           string            `json:"scope"`
	Subnet          string            `json:"subnet,omitempty"`
	Gateway         string            `json:"gateway,omitempty"`
	Internal        bool              `json:"internal"`
	EnableIPv6      bool              `json:"enableIPv6"`
	ContainersCount int               `json:"containersCount"`
	Containers      map[string]string `json:"containers,omitempty"` // container ID/Name -> IP
	Created         string            `json:"created,omitempty"`
}

type CreateNetworkRequest struct {
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Subnet     string `json:"subnet,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
	Internal   bool   `json:"internal"`
	EnableIPv6 bool   `json:"enableIPv6"`
}

type DeployContainerRequest struct {
	Name           string   `json:"name"`
	Image          string   `json:"image"`
	PortBindings   []string `json:"portBindings"`   // e.g. ["8080:80", "443:443"]
	VolumeBindings []string `json:"volumeBindings"` // e.g. ["/data:/app/data"]
	EnvVars        []string `json:"envVars"`        // e.g. ["FOO=BAR"]
	RestartPolicy  string   `json:"restartPolicy"`  // "always", "unless-stopped", "no"
	Command        string   `json:"command,omitempty"`
}

type DockerSystemInfo struct {
	ServerVersion     string  `json:"serverVersion"`
	Containers        int     `json:"containers"`
	ContainersRunning int     `json:"containersRunning"`
	ContainersPaused  int     `json:"containersPaused"`
	ContainersStopped int     `json:"containersStopped"`
	Images            int     `json:"images"`
	OperatingSystem   string  `json:"operatingSystem"`
	OSType            string  `json:"osType"`
	Architecture      string  `json:"architecture"`
	NCPU              int     `json:"ncpu"`
	MemTotalMB        float64 `json:"memTotalMb"`
}

// ==================== VISUAL REPORTS DOMAIN ====================

type VisualReportHeaderConfig struct {
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"`
	ShowDate   bool   `json:"showDate"`
	LogoText   string `json:"logoText"`
	TotalPages int    `json:"totalPages,omitempty"`
}

type VisualReport struct {
	ID              string                   `json:"id"`
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	Mode            string                   `json:"mode"`            // "document" (multi-page A4) or "grid" (dashboard)
	PageOrientation string                   `json:"pageOrientation"` // "portrait" or "landscape"
	HeaderConfig    VisualReportHeaderConfig `json:"headerConfig"`
	UserID          *int                     `json:"userId,omitempty"`
	Widgets         []VisualReportWidget     `json:"widgets,omitempty"`
	CreatedAt       time.Time                `json:"createdAt"`
	UpdatedAt       time.Time                `json:"updatedAt"`
}

type VisualReportWidget struct {
	ID           string                 `json:"id"`
	ReportID     string                 `json:"reportId"`
	PageNumber   int                    `json:"pageNumber"`
	Title        string                 `json:"title"`
	ChartType    string                 `json:"chartType"`  // "line", "bar", "area", "gauge", "table", "metric_card"
	SourceType   string                 `json:"sourceType"` // "opensearch", "grafana"
	SourceConfig map[string]interface{} `json:"sourceConfig"`
	TimeRange    string                 `json:"timeRange"` // "1h", "24h", "7d", "30d"
	Theme        string                 `json:"theme"`
	WidthPercent int                    `json:"widthPercent"` // 50 (Half Width), 100 (Full Width)
	SortOrder    int                    `json:"sortOrder"`
	CreatedAt    time.Time              `json:"createdAt"`
}

type ReportDataPoint struct {
	Timestamp string  `json:"timestamp"`
	Label     string  `json:"label"`
	Value     float64 `json:"value"`
}

type ReportWidgetSummary struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Avg      float64 `json:"avg"`
	Current  float64 `json:"current"`
	Total    float64 `json:"total"`
	Count    int     `json:"count"`
	Unit     string  `json:"unit"`
	PeakTime string  `json:"peakTime,omitempty"`
}

type ReportQueryDataRequest struct {
	SourceType   string                 `json:"sourceType"` // "opensearch", "grafana"
	SourceConfig map[string]interface{} `json:"sourceConfig"`
	TimeRange    string                 `json:"timeRange"` // "1h", "24h", "7d", "30d"
	MetricKey    string                 `json:"metricKey,omitempty"`
}

type ReportSeries struct {
	Name    string              `json:"name"`
	Host    string              `json:"host,omitempty"`
	Points  []ReportDataPoint   `json:"points"`
	Summary ReportWidgetSummary `json:"summary,omitempty"`
}

type ReportQueryDataResponse struct {
	Title       string              `json:"title"`
	SourceType  string              `json:"sourceType"`
	Points      []ReportDataPoint   `json:"points"`
	Summary     ReportWidgetSummary `json:"summary"`
	Series      []ReportSeries      `json:"series,omitempty"`
	Categories  []string            `json:"categories,omitempty"`
	TableRows   []map[string]any    `json:"tableRows,omitempty"`
	IsConnected bool                `json:"isConnected"`
	Message     string              `json:"message,omitempty"`
}

// ==================== STATUS PAGES DOMAIN ====================

type StatusPage struct {
	ID              string               `json:"id"`
	Title           string               `json:"title"`
	Slug            string               `json:"slug"`
	Description     string               `json:"description"`
	FooterText      string               `json:"footerText"`
	Theme           string               `json:"theme"` // "auto", "dark", "light"
	RefreshInterval int                  `json:"refreshInterval"` // in seconds, default 60
	IsPublic        bool                 `json:"isPublic"`
	IsPublished     bool                 `json:"isPublished"`
	ShowTags        bool                 `json:"showTags"`
	CustomCSS       string               `json:"customCss"`
	UserID          *int                 `json:"userId,omitempty"`
	Groups          []StatusPageGroup    `json:"groups,omitempty"`
	Items           []StatusPageItem     `json:"items,omitempty"`
	Incidents       []StatusPageIncident `json:"incidents,omitempty"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
}

type StatusPageGroup struct {
	ID        string           `json:"id"`
	PageID    string           `json:"pageId"`
	Name      string           `json:"name"`
	SortOrder int              `json:"sortOrder"`
	Items     []StatusPageItem `json:"items,omitempty"`
	CreatedAt time.Time        `json:"createdAt"`
}

type StatusPageItem struct {
	ID           string                 `json:"id"`
	PageID       string                 `json:"pageId"`
	GroupID      *string                `json:"groupId,omitempty"`
	Name         string                 `json:"name"`
	SourceType   string                 `json:"sourceType"` // "topology", "opensearch", "prometheus", "grafana", "remote_server"
	SourceID     *string                `json:"sourceId,omitempty"`
	SourceConfig map[string]interface{} `json:"sourceConfig"`
	Description  string                 `json:"description"`
	SortOrder    int                    `json:"sortOrder"`
	CreatedAt    time.Time              `json:"createdAt"`
}

type StatusPageIncident struct {
	ID        string    `json:"id"`
	PageID    string    `json:"pageId"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // "investigating", "identified", "monitoring", "resolved", "maintenance"
	Severity  string    `json:"severity"` // "info", "minor", "major", "critical"
	Message   string    `json:"message"`
	IsActive  bool      `json:"isActive"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type StatusItemLiveResult struct {
	ItemID      string                 `json:"itemId"`
	Name        string                 `json:"name"`
	GroupID     *string                `json:"groupId,omitempty"`
	SourceType  string                 `json:"sourceType"`
	SourceID    *string                `json:"sourceId,omitempty"`
	Status      string                 `json:"status"` // "operational", "degraded", "down", "maintenance", "unknown"
	LatencyMs   *float64               `json:"latencyMs,omitempty"`
	Message     string                 `json:"message,omitempty"`
	Description string                 `json:"description,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	CheckedAt   time.Time              `json:"checkedAt"`
}

type StatusPageGroupReport struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	SortOrder int                    `json:"sortOrder"`
	Items     []StatusItemLiveResult `json:"items"`
}

type StatusPageLiveReport struct {
	PageID          string                  `json:"pageId"`
	Title           string                  `json:"title"`
	Slug            string                  `json:"slug"`
	Description     string                  `json:"description"`
	FooterText      string                  `json:"footerText"`
	Theme           string                  `json:"theme"`
	RefreshInterval int                     `json:"refreshInterval"`
	IsPublic        bool                    `json:"isPublic"`
	OverallStatus   string                  `json:"overallStatus"` // "operational", "partial_outage", "major_outage", "maintenance"
	OverallMessage  string                  `json:"overallMessage"`
	ActiveIncidents []StatusPageIncident    `json:"activeIncidents"`
	Groups          []StatusPageGroupReport `json:"groups"`
	UngroupedItems  []StatusItemLiveResult  `json:"ungroupedItems,omitempty"`
	LastChecked     time.Time               `json:"lastChecked"`
}

type StatusPageSourceOption struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourceType string `json:"sourceType"` // "topology", "opensearch", "prometheus", "grafana", "remote_server"
	Detail     string `json:"detail,omitempty"`
	IPOrHost   string `json:"ipOrHost,omitempty"`
	Status     string `json:"status,omitempty"`
}


