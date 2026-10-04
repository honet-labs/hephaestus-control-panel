<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import axios from 'axios';
import {
  Boxes,
  Search,
  X,
  ArrowLeft,
  Server,
  Terminal,
  FileCode,
  Copy,
  Check,
  ExternalLink,
  AlertCircle,
  Link2,
  RefreshCw,
  FolderArchive,
  Layers,
  Shield,
  Activity,
  Cpu,
  Database,
  Sliders,
  CheckCircle2,
  Play,
} from 'lucide-vue-next';

interface DockerConnection {
  id: string;
  name: string;
  host: string;
  hostType: string;
  isDefault?: boolean;
}

interface SystemdInstall {
  serviceName: string;
  installCmd: string;
  serviceUnit: string;
  manageCmd: string;
}

interface AppItem {
  id: string;
  name: string;
  category: string;
  tag: string;
  summary: string;
  officialDocs: string;
  defaultImage: string;
  defaultContainerName: string;
  defaultPorts: string[];
  defaultVolumes: string[];
  defaultEnv: string[];
  restartPolicy: string;
  manualInstall: {
    portsDesc: string;
    runCmd: string;
    composeYaml: string;
    systemd: SystemdInstall;
    hcpSteps: string[];
  };
}

const props = defineProps<{
  isOpen: boolean;
}>();

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const router = useRouter();

// Active state
const searchQuery = ref('');
const selectedCategory = ref('all');
const selectedApp = ref<AppItem | null>(null);
const activeTab = ref<'docker' | 'manual'>('docker');
const manualMethod = ref<'docker' | 'systemd'>('docker');

// Docker connections & deployment state
const dockerConnections = ref<DockerConnection[]>([]);
const loadingConnections = ref(false);
const connectionsError = ref('');
const selectedConnectionId = ref('');

// Deployment form
const deployForm = ref({
  name: '',
  image: '',
  portsText: '',
  volumesText: '',
  envVarsText: '',
  restartPolicy: 'unless-stopped',
});

const isDeploying = ref(false);
const deploySuccess = ref(false);
const deployResultId = ref('');
const deployError = ref('');
const showNoConnectionModal = ref(false);
const copiedCodeType = ref<string | null>(null);

// App catalog list
const apps: AppItem[] = [
  {
    id: 'grafana',
    name: 'Grafana',
    category: 'Observability » Dashboards & Analytics',
    tag: 'Observability',
    summary: 'The open and composable observability and data visualization platform. Visualize metrics, logs, and traces from Prometheus, OpenSearch, and multiple data sources.',
    officialDocs: 'https://grafana.com/docs/',
    defaultImage: 'grafana/grafana:latest',
    defaultContainerName: 'hephaestus-grafana',
    defaultPorts: ['3000:3000'],
    defaultVolumes: ['grafana-data:/var/lib/grafana'],
    defaultEnv: [
      'GF_SECURITY_ADMIN_USER=admin',
      'GF_SECURITY_ADMIN_PASSWORD=admin',
      'GF_USERS_ALLOW_SIGN_UP=false',
    ],
    restartPolicy: 'unless-stopped',
    manualInstall: {
      portsDesc: 'Port 3000 for Grafana HTTP web dashboard',
      runCmd: `docker run -d \\
  --name hephaestus-grafana \\
  --restart unless-stopped \\
  -p 3000:3000 \\
  -v grafana-data:/var/lib/grafana \\
  -e GF_SECURITY_ADMIN_USER=admin \\
  -e GF_SECURITY_ADMIN_PASSWORD=admin \\
  grafana/grafana:latest`,
      composeYaml: `version: '3.8'
services:
  grafana:
    image: grafana/grafana:latest
    container_name: hephaestus-grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    volumes:
      - grafana-data:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false

volumes:
  grafana-data:`,
      systemd: {
        serviceName: 'grafana-server.service',
        installCmd: `# 1. Install prerequisites & official Grafana APT repository
sudo apt-get install -y apt-transport-https software-properties-common wget
sudo mkdir -p /etc/apt/keyrings/
wget -q -O - https://apt.grafana.com/gpg.key | gpg --dearmor | sudo tee /etc/apt/keyrings/grafana.gpg > /dev/null
echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" | sudo tee /etc/apt/sources.list.d/grafana.list

# 2. Install Grafana Enterprise / OSS
sudo apt-get update && sudo apt-get install -y grafana`,
        serviceUnit: `[Unit]
Description=Grafana instance
Documentation=http://docs.grafana.org
Wants=network-online.target
After=network-online.target

[Service]
User=grafana
Group=grafana
Type=simple
Restart=on-failure
WorkingDirectory=/usr/share/grafana
RuntimeDirectory=grafana
RuntimeDirectoryMode=0750
ExecStart=/usr/sbin/grafana-server \\
  --config=/etc/grafana/grafana.ini \\
  --homepath=/usr/share/grafana \\
  --packaging=deb \\
  cfg:default.paths.logs=/var/log/grafana \\
  cfg:default.paths.data=/var/lib/grafana \\
  cfg:default.paths.plugins=/var/lib/grafana/plugins
LimitNOFILE=10000
TimeoutStopSec=20

[Install]
WantedBy=multi-user.target`,
        manageCmd: `sudo systemctl daemon-reload
sudo systemctl enable --now grafana-server
sudo systemctl status grafana-server`,
      },
      hcpSteps: [
        'Access the Grafana web UI in your browser at http://<host-ip>:3000 using the default credentials admin / admin.',
        'Navigate to Connections » Data Sources » Add new data source in Grafana.',
        'Select Prometheus and provide the Prometheus server URL (e.g., http://<prometheus-ip>:9090) to query metrics collected from Hephaestus agent.',
        'Select OpenSearch and provide your OpenSearch cluster URL (e.g., http://<opensearch-ip>:9200) for log analytics visualization.',
        'Grafana dashboard panels and shared links can be directly embedded into Hephaestus Control Panel via Slide Show or Visual Reports.',
      ],
    },
  },
  {
    id: 'data-prepper',
    name: 'Data Prepper',
    category: 'Data Pipelines » Log Ingestion & Processing',
    tag: 'Pipelines',
    summary: 'Server-side data collector and transformation engine that filters, extracts, enriches, and buffers logs/traces before routing into OpenSearch.',
    officialDocs: 'https://opensearch.org/docs/latest/data-prepper/',
    defaultImage: 'opensearchproject/data-prepper:latest',
    defaultContainerName: 'hephaestus-data-prepper',
    defaultPorts: ['2021:2021', '4900:4900'],
    defaultVolumes: ['dataprepper-pipelines:/usr/share/data-prepper/pipelines'],
    defaultEnv: ['DATA_PREPPER_LOG_LEVEL=INFO'],
    restartPolicy: 'unless-stopped',
    manualInstall: {
      portsDesc: 'Port 2021 for API / health check, Port 4900 for log pipelines',
      runCmd: `docker run -d \\
  --name hephaestus-data-prepper \\
  --restart unless-stopped \\
  -p 2021:2021 \\
  -p 4900:4900 \\
  -v dataprepper-pipelines:/usr/share/data-prepper/pipelines \\
  -e DATA_PREPPER_LOG_LEVEL=INFO \\
  opensearchproject/data-prepper:latest`,
      composeYaml: `version: '3.8'
services:
  data-prepper:
    image: opensearchproject/data-prepper:latest
    container_name: hephaestus-data-prepper
    restart: unless-stopped
    ports:
      - "2021:2021"
      - "4900:4900"
    volumes:
      - dataprepper-pipelines:/usr/share/data-prepper/pipelines
    environment:
      - DATA_PREPPER_LOG_LEVEL=INFO

volumes:
  dataprepper-pipelines:`,
      systemd: {
        serviceName: 'data-prepper.service',
        installCmd: `# 1. Install Java 17 Runtime & prerequisites
sudo apt-get update && sudo apt-get install -y openjdk-17-jre-headless curl tar

# 2. Create system user and directory structure
sudo useradd --system --no-create-home --shell /bin/false dataprepper
sudo mkdir -p /opt/data-prepper /etc/data-prepper/pipelines /var/log/data-prepper

# 3. Download & extract Data Prepper release
DATA_PREPPER_VERSION="2.11.0"
curl -fsSL "https://d2s0cvqjn7530p.cloudfront.net/tarball/data-prepper/data-prepper-\${DATA_PREPPER_VERSION}-linux-x64.tar.gz" -o /tmp/data-prepper.tar.gz
sudo tar -xzf /tmp/data-prepper.tar.gz -C /opt/data-prepper --strip-components=1
rm -f /tmp/data-prepper.tar.gz
sudo chown -R dataprepper:dataprepper /opt/data-prepper /etc/data-prepper /var/log/data-prepper`,
        serviceUnit: `[Unit]
Description=OpenSearch Data Prepper Pipeline Ingestion Service
Documentation=https://opensearch.org/docs/latest/data-prepper/
After=network.target

[Service]
Type=simple
User=dataprepper
Group=dataprepper
WorkingDirectory=/opt/data-prepper
Environment="DATA_PREPPER_LOG_LEVEL=INFO"
Environment="JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64"
ExecStart=/opt/data-prepper/bin/data-prepper /etc/data-prepper/pipelines/pipelines.yaml
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target`,
        manageCmd: `sudo systemctl daemon-reload
sudo systemctl enable --now data-prepper
sudo systemctl status data-prepper`,
      },
      hcpSteps: [
        'Open the Data Prepper Pipelines menu in the Hephaestus Control Panel sidebar.',
        'Use the integrated YAML pipeline editor to configure ingestion sources (OTLP, HTTP, Logstash buffer) and target sinks to OpenSearch.',
        'Hephaestus automatically validates Data Prepper pipeline YAML syntax and schema formatting.',
        'Use the volume mount dataprepper-pipelines or the /etc/data-prepper/pipelines directory to synchronize pipeline configuration files with the Data Prepper daemon or container.',
      ],
    },
  },
  {
    id: 'opentelemetry',
    name: 'OpenTelemetry Collector',
    category: 'Telemetry » Traces, Metrics & Logs',
    tag: 'Telemetry',
    summary: 'Vendor-agnostic proxy that receives, processes, batches, and exports telemetry data across microservices, infrastructure nodes, and cloud systems.',
    officialDocs: 'https://opentelemetry.io/docs/collector/',
    defaultImage: 'otel/opentelemetry-collector-contrib:latest',
    defaultContainerName: 'hephaestus-otel-collector',
    defaultPorts: ['4317:4317', '4318:4318', '8889:8889'],
    defaultVolumes: ['otel-collector-config:/etc/otelcol-contrib'],
    defaultEnv: ['OTEL_LOG_LEVEL=info'],
    restartPolicy: 'unless-stopped',
    manualInstall: {
      portsDesc: 'Port 4317 (gRPC OTLP), Port 4318 (HTTP OTLP), Port 8889 (Prometheus exporter)',
      runCmd: `docker run -d \\
  --name hephaestus-otel-collector \\
  --restart unless-stopped \\
  -p 4317:4317 \\
  -p 4318:4318 \\
  -p 8889:8889 \\
  -v otel-collector-config:/etc/otelcol-contrib \\
  otel/opentelemetry-collector-contrib:latest`,
      composeYaml: `version: '3.8'
services:
  otel-collector:
    image: otel/opentelemetry-collector-contrib:latest
    container_name: hephaestus-otel-collector
    restart: unless-stopped
    ports:
      - "4317:4317"
      - "4318:4318"
      - "8889:8889"
    volumes:
      - otel-collector-config:/etc/otelcol-contrib

volumes:
  otel-collector-config:`,
      systemd: {
        serviceName: 'otelcol-contrib.service',
        installCmd: `# 1. Download official OpenTelemetry Collector Contrib Debian package
OTEL_VERSION="0.108.0"
curl -fsSL -O "https://github.com/open-telemetry/opentelemetry-collector-releases/releases/download/v\${OTEL_VERSION}/otelcol-contrib_\${OTEL_VERSION}_linux_amd64.deb"

# 2. Install package via dpkg (or rpm for RHEL/CentOS)
sudo dpkg -i "otelcol-contrib_\${OTEL_VERSION}_linux_amd64.deb"
rm -f "otelcol-contrib_\${OTEL_VERSION}_linux_amd64.deb"`,
        serviceUnit: `[Unit]
Description=OpenTelemetry Collector Contrib
Documentation=https://opentelemetry.io/docs/collector/
After=network.target

[Service]
User=otelcol-contrib
Group=otelcol-contrib
ExecStart=/usr/bin/otelcol-contrib --config=/etc/otelcol-contrib/config.yaml
Restart=always
RestartSec=5
KillMode=mixed
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target`,
        manageCmd: `sudo systemctl daemon-reload
sudo systemctl enable --now otelcol-contrib
sudo systemctl status otelcol-contrib`,
      },
      hcpSteps: [
        'Open the OpenTelemetry Config menu in the Hephaestus Control Panel sidebar.',
        'Customize required receivers, batch processors, memory limiters, and exporters.',
        'Configure your applications or server agents to send OTLP telemetry to http://<host-ip>:4317 (gRPC) or http://<host-ip>:4318 (HTTP).',
        'Telemetry metrics and traces collected by the collector can be forwarded to Prometheus or OpenSearch registered in Hephaestus.',
      ],
    },
  },
  {
    id: 'vaultwarden',
    name: 'Vaultwarden',
    category: 'Security & Secrets » Password & Secret Manager',
    tag: 'Security',
    summary: 'Lightweight Bitwarden-compatible server written in Rust. Securely manage passwords, SSH credentials, API secrets, and server certificates.',
    officialDocs: 'https://github.com/dani-garcia/vaultwarden',
    defaultImage: 'vaultwarden/server:latest',
    defaultContainerName: 'hephaestus-vaultwarden',
    defaultPorts: ['8080:80'],
    defaultVolumes: ['vaultwarden-data:/data'],
    defaultEnv: ['SIGNUPS_ALLOWED=true', 'WEBSOCKET_ENABLED=true'],
    restartPolicy: 'unless-stopped',
    manualInstall: {
      portsDesc: 'Port 8080 (Host) mapped to Port 80 (Container Web UI)',
      runCmd: `docker run -d \\
  --name hephaestus-vaultwarden \\
  --restart unless-stopped \\
  -p 8080:80 \\
  -v vaultwarden-data:/data \\
  -e SIGNUPS_ALLOWED=true \\
  -e WEBSOCKET_ENABLED=true \\
  vaultwarden/server:latest`,
      composeYaml: `version: '3.8'
services:
  vaultwarden:
    image: vaultwarden/server:latest
    container_name: hephaestus-vaultwarden
    restart: unless-stopped
    ports:
      - "8080:80"
    volumes:
      - vaultwarden-data:/data
    environment:
      - SIGNUPS_ALLOWED=true
      - WEBSOCKET_ENABLED=true

volumes:
  vaultwarden-data:`,
      systemd: {
        serviceName: 'vaultwarden.service',
        installCmd: `# 1. Create vaultwarden system user and data folders
sudo useradd --system --shell /bin/false --no-create-home vaultwarden
sudo mkdir -p /var/lib/vaultwarden/data /etc/vaultwarden

# 2. Download precompiled Vaultwarden Linux binary release
sudo curl -fsSL -o /usr/local/bin/vaultwarden https://github.com/dani-garcia/vaultwarden/releases/latest/download/vaultwarden-linux-x86_64
sudo chmod +x /usr/local/bin/vaultwarden
sudo chown -R vaultwarden:vaultwarden /var/lib/vaultwarden /etc/vaultwarden`,
        serviceUnit: `[Unit]
Description=Vaultwarden Password Manager Service
Documentation=https://github.com/dani-garcia/vaultwarden
After=network.target

[Service]
User=vaultwarden
Group=vaultwarden
WorkingDirectory=/var/lib/vaultwarden
Environment="DATA_FOLDER=/var/lib/vaultwarden/data"
Environment="ROCKET_PORT=8080"
Environment="ROCKET_ADDRESS=0.0.0.0"
Environment="SIGNUPS_ALLOWED=true"
Environment="WEBSOCKET_ENABLED=true"
ExecStart=/usr/local/bin/vaultwarden
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target`,
        manageCmd: `sudo systemctl daemon-reload
sudo systemctl enable --now vaultwarden
sudo systemctl status vaultwarden`,
      },
      hcpSteps: [
        'Open the Vaultwarden web interface at http://<host-ip>:8080 and create your primary administrator account.',
        'Navigate to the Vaultwarden menu in the Hephaestus sidebar (under the Security group).',
        'Connect the Vaultwarden API endpoint or access token to enable automated secret synchronization with Hephaestus Remote Server and Connections.',
        'All server SSH credentials and database passwords can now be retrieved dynamically from your encrypted vault.',
      ],
    },
  },
  {
    id: 'prometheus',
    name: 'Prometheus',
    category: 'Monitoring » Time Series Metrics Engine',
    tag: 'Monitoring',
    summary: 'Open-source systems monitoring and alerting toolkit. Scrapes metrics from host targets, node exporters, and microservices at high frequency.',
    officialDocs: 'https://prometheus.io/docs/',
    defaultImage: 'prom/prometheus:latest',
    defaultContainerName: 'hephaestus-prometheus',
    defaultPorts: ['9090:9090'],
    defaultVolumes: [
      'prometheus-data:/prometheus',
      'prometheus-config:/etc/prometheus',
    ],
    defaultEnv: [],
    restartPolicy: 'unless-stopped',
    manualInstall: {
      portsDesc: 'Port 9090 for Prometheus Web UI and Query API',
      runCmd: `docker run -d \\
  --name hephaestus-prometheus \\
  --restart unless-stopped \\
  -p 9090:9090 \\
  -v prometheus-data:/prometheus \\
  -v prometheus-config:/etc/prometheus \\
  prom/prometheus:latest \\
  --config.file=/etc/prometheus/prometheus.yml \\
  --storage.tsdb.path=/prometheus \\
  --web.enable-lifecycle`,
      composeYaml: `version: '3.8'
services:
  prometheus:
    image: prom/prometheus:latest
    container_name: hephaestus-prometheus
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - prometheus-data:/prometheus
      - prometheus-config:/etc/prometheus
    command:
      - "--config.file=/etc/prometheus/prometheus.yml"
      - "--storage.tsdb.path=/prometheus"
      - "--web.enable-lifecycle"

volumes:
  prometheus-data:
  prometheus-config:`,
      systemd: {
        serviceName: 'prometheus.service',
        installCmd: `# 1. Create prometheus system user and directories
sudo useradd --no-create-home --shell /bin/false prometheus
sudo mkdir -p /etc/prometheus /var/lib/prometheus

# 2. Download and unpack Prometheus official release
PROM_VERSION="2.54.1"
curl -fsSL -O "https://github.com/prometheus/prometheus/releases/download/v\${PROM_VERSION}/prometheus-\${PROM_VERSION}.linux-amd64.tar.gz"
tar -xzf "prometheus-\${PROM_VERSION}.linux-amd64.tar.gz"
sudo cp "prometheus-\${PROM_VERSION}.linux-amd64/prometheus" /usr/local/bin/
sudo cp "prometheus-\${PROM_VERSION}.linux-amd64/promtool" /usr/local/bin/
sudo cp -r "prometheus-\${PROM_VERSION}.linux-amd64/consoles" /etc/prometheus/
sudo cp -r "prometheus-\${PROM_VERSION}.linux-amd64/console_libraries" /etc/prometheus/
sudo cp "prometheus-\${PROM_VERSION}.linux-amd64/prometheus.yml" /etc/prometheus/prometheus.yml
rm -rf "prometheus-\${PROM_VERSION}.linux-amd64"*
sudo chown -R prometheus:prometheus /etc/prometheus /var/lib/prometheus /usr/local/bin/prometheus /usr/local/bin/promtool`,
        serviceUnit: `[Unit]
Description=Prometheus Time Series Monitoring Service
Documentation=https://prometheus.io/docs/introduction/overview/
Wants=network-online.target
After=network-online.target

[Service]
User=prometheus
Group=prometheus
Type=simple
ExecStart=/usr/local/bin/prometheus \\
  --config.file=/etc/prometheus/prometheus.yml \\
  --storage.tsdb.path=/var/lib/prometheus/ \\
  --web.console.templates=/etc/prometheus/consoles \\
  --web.console.libraries=/etc/prometheus/console_libraries \\
  --web.enable-lifecycle
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target`,
        manageCmd: `sudo systemctl daemon-reload
sudo systemctl enable --now prometheus
sudo systemctl status prometheus`,
      },
      hcpSteps: [
        'Open the Prometheus Config menu in the Hephaestus Control Panel sidebar.',
        'Configure prometheus.yml to define scrape intervals and scrape targets (such as node_exporter on port 9100).',
        'Navigate to Monitoring Instances in Hephaestus to enable host telemetry monitoring powered by Prometheus metrics.',
        'Hephaestus can update and dynamically reload Prometheus configurations via HTTP without service restarts thanks to the --web.enable-lifecycle flag.',
      ],
    },
  },
  {
    id: 'opensearch',
    name: 'OpenSearch',
    category: 'Search & Analytics » Search Engine & Log Store',
    tag: 'Analytics',
    summary: 'Scalable distributed search and analytics suite. Stores, searches, and visualizes system logs, security audit records, and observability events.',
    officialDocs: 'https://opensearch.org/docs/',
    defaultImage: 'opensearchproject/opensearch:latest',
    defaultContainerName: 'hephaestus-opensearch',
    defaultPorts: ['9200:9200', '9600:9600'],
    defaultVolumes: ['opensearch-data:/usr/share/opensearch/data'],
    defaultEnv: [
      'discovery.type=single-node',
      'DISABLE_SECURITY_PLUGIN=true',
      'OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m',
    ],
    restartPolicy: 'unless-stopped',
    manualInstall: {
      portsDesc: 'Port 9200 for OpenSearch REST API, Port 9600 for Performance Analyzer',
      runCmd: `docker run -d \\
  --name hephaestus-opensearch \\
  --restart unless-stopped \\
  -p 9200:9200 \\
  -p 9600:9600 \\
  -v opensearch-data:/usr/share/opensearch/data \\
  -e "discovery.type=single-node" \\
  -e "DISABLE_SECURITY_PLUGIN=true" \\
  -e "OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m" \\
  opensearchproject/opensearch:latest`,
      composeYaml: `version: '3.8'
services:
  opensearch:
    image: opensearchproject/opensearch:latest
    container_name: hephaestus-opensearch
    restart: unless-stopped
    ports:
      - "9200:9200"
      - "9600:9600"
    volumes:
      - opensearch-data:/usr/share/opensearch/data
    environment:
      - discovery.type=single-node
      - DISABLE_SECURITY_PLUGIN=true
      - OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m

volumes:
  opensearch-data:`,
      systemd: {
        serviceName: 'opensearch.service',
        installCmd: `# 1. Configure system kernel limits required by OpenSearch
sudo sysctl -w vm.max_map_count=262144
echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.d/99-opensearch.conf

# 2. Create opensearch system user and folders
sudo useradd --system --shell /bin/false --no-create-home opensearch
sudo mkdir -p /opt/opensearch /var/lib/opensearch /var/log/opensearch

# 3. Download & extract OpenSearch release bundle
OS_VERSION="2.16.0"
curl -fsSL -O "https://artifacts.opensearch.org/releases/bundle/opensearch/\${OS_VERSION}/opensearch-\${OS_VERSION}-linux-x64.tar.gz"
sudo tar -xzf "opensearch-\${OS_VERSION}-linux-x64.tar.gz" -C /opt/opensearch --strip-components=1
rm -f "opensearch-\${OS_VERSION}-linux-x64.tar.gz"
sudo chown -R opensearch:opensearch /opt/opensearch /var/lib/opensearch /var/log/opensearch`,
        serviceUnit: `[Unit]
Description=OpenSearch Distributed Search & Analytics Engine
Documentation=https://opensearch.org/docs/latest/
After=network.target

[Service]
Type=simple
User=opensearch
Group=opensearch
WorkingDirectory=/opt/opensearch
Environment="OPENSEARCH_HOME=/opt/opensearch"
Environment="OPENSEARCH_PATH_CONF=/opt/opensearch/config"
Environment="OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m"
ExecStart=/opt/opensearch/bin/opensearch
StandardOutput=journal
StandardError=journal
LimitNOFILE=65536
LimitNPROC=4096
LimitMEMLOCK=infinity
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`,
        manageCmd: `sudo systemctl daemon-reload
sudo systemctl enable --now opensearch
sudo systemctl status opensearch`,
      },
      hcpSteps: [
        'Open the OpenSearch Cluster menu in the Hephaestus Control Panel sidebar.',
        'Monitor cluster health status (Green/Yellow/Red), indexed document counts, and shard memory allocation in real time.',
        'Connect Data Prepper pipelines to automatically parse, enrich, and index incoming log streams into OpenSearch indices.',
        'Utilize the Raw Data Reports module and log analytics visualizations to inspect security events and system anomalies.',
      ],
    },
  },
];

// Categories for filtering
const categories = [
  { id: 'all', label: 'All Apps' },
  { id: 'Observability', label: 'Observability' },
  { id: 'Pipelines', label: 'Pipelines' },
  { id: 'Telemetry', label: 'Telemetry' },
  { id: 'Security', label: 'Security' },
  { id: 'Monitoring', label: 'Monitoring' },
  { id: 'Analytics', label: 'Analytics' },
];

const filteredApps = computed(() => {
  let list = apps;
  if (selectedCategory.value !== 'all') {
    list = list.filter((app) => app.tag === selectedCategory.value);
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim();
    list = list.filter(
      (app) =>
        app.name.toLowerCase().includes(q) ||
        app.category.toLowerCase().includes(q) ||
        app.summary.toLowerCase().includes(q) ||
        app.defaultImage.toLowerCase().includes(q)
    );
  }
  return list;
});

// Load Docker connections
const fetchDockerConnections = async () => {
  loadingConnections.value = true;
  connectionsError.value = '';
  try {
    const res = await axios.get('/api/v1/docker/connections');
    if (res.data?.success && Array.isArray(res.data.data)) {
      dockerConnections.value = res.data.data;
      if (dockerConnections.value.length > 0 && !selectedConnectionId.value) {
        const def = dockerConnections.value.find((c) => c.isDefault);
        selectedConnectionId.value = def ? def.id : dockerConnections.value[0].id;
      }
    } else {
      dockerConnections.value = [];
    }
  } catch (err: any) {
    connectionsError.value =
      err.response?.data?.error || 'Failed to load Docker connections';
    dockerConnections.value = [];
  } finally {
    loadingConnections.value = false;
  }
};

// Open App Details
const openAppDetail = (app: AppItem) => {
  selectedApp.value = app;
  activeTab.value = 'docker';
  manualMethod.value = 'docker';
  deploySuccess.value = false;
  deployError.value = '';
  deployResultId.value = '';

  // Pre-fill deploy form
  deployForm.value = {
    name: app.defaultContainerName,
    image: app.defaultImage,
    portsText: app.defaultPorts.join('\n'),
    volumesText: app.defaultVolumes.join('\n'),
    envVarsText: app.defaultEnv.join('\n'),
    restartPolicy: app.restartPolicy || 'unless-stopped',
  };
};

const backToCatalog = () => {
  selectedApp.value = null;
  deploySuccess.value = false;
  deployError.value = '';
};

// Copy code to clipboard
const copyCode = async (text: string, type: string) => {
  try {
    await navigator.clipboard.writeText(text);
    copiedCodeType.value = type;
    setTimeout(() => {
      if (copiedCodeType.value === type) {
        copiedCodeType.value = null;
      }
    }, 2000);
  } catch {
    // Fallback
  }
};

// Go to connections menu
const goToConnections = () => {
  emit('close');
  router.push('/connections');
};

// Navigate to container management
const goToContainers = () => {
  emit('close');
  router.push('/infrastructure/containers');
};

// Execute docker deploy
const handleInstallToDocker = async () => {
  if (dockerConnections.value.length === 0) {
    showNoConnectionModal.value = true;
    return;
  }

  if (!selectedConnectionId.value) {
    deployError.value = 'Please select a target Docker Connection first.';
    return;
  }

  isDeploying.value = true;
  deployError.value = '';
  deploySuccess.value = false;

  const ports = deployForm.value.portsText
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);

  const volumes = deployForm.value.volumesText
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);

  const envVars = deployForm.value.envVarsText
    .split('\n')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);

  const payload = {
    name: deployForm.value.name.trim(),
    image: deployForm.value.image.trim(),
    portBindings: ports,
    volumeBindings: volumes,
    envVars: envVars,
    restartPolicy: deployForm.value.restartPolicy || 'unless-stopped',
    visibility: 'private',
  };

  try {
    const res = await axios.post(
      `/api/v1/docker/containers/deploy?connectionId=${selectedConnectionId.value}`,
      payload
    );

    if (res.data?.success) {
      deploySuccess.value = true;
      deployResultId.value = res.data.containerId || 'OK';
    } else {
      deployError.value = res.data?.error || 'Failed to deploy container';
    }
  } catch (err: any) {
    deployError.value =
      err.response?.data?.error ||
      err.response?.data?.details ||
      err.message ||
      'Deployment failed. Please check Docker daemon logs and connection.';
  } finally {
    isDeploying.value = false;
  }
};

// Watch for modal open
watch(
  () => props.isOpen,
  (val) => {
    if (val) {
      fetchDockerConnections();
      searchQuery.value = '';
      selectedCategory.value = 'all';
      selectedApp.value = null;
      deploySuccess.value = false;
      deployError.value = '';
      showNoConnectionModal.value = false;
    }
  }
);

const handleKeyDown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.isOpen) {
    if (showNoConnectionModal.value) {
      showNoConnectionModal.value = false;
    } else if (selectedApp.value) {
      backToCatalog();
    } else {
      emit('close');
    }
  }
};

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown);
});

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div
    v-if="isOpen"
    class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in duration-150"
    role="dialog"
    aria-modal="true"
  >
    <!-- Modal Card Container -->
    <div
      class="bg-white dark:bg-[#0f1422] border border-slate-200 dark:border-[#1b2234] rounded-2xl w-full max-w-5xl h-[88vh] flex flex-col shadow-2xl overflow-hidden font-sans"
    >
      <!-- Top Header Bar -->
      <div
        class="flex items-center justify-between px-5 py-4 border-b border-slate-200 dark:border-[#1b2234] bg-slate-50/70 dark:bg-[#121827]/70 shrink-0"
      >
        <div class="flex items-center gap-3 min-w-0">
          <div
            class="w-9 h-9 rounded-xl bg-slate-100 dark:bg-[#1a2336] border border-slate-200 dark:border-[#222e47] flex items-center justify-center text-slate-600 dark:text-slate-300 shrink-0"
          >
            <Boxes class="w-5 h-5 text-slate-500 dark:text-slate-400" />
          </div>
          <div>
            <div class="flex items-center gap-2">
              <h2 class="text-base font-bold text-slate-900 dark:text-white leading-tight">
                App Library
              </h2>
              <span
                class="px-2 py-0.5 text-[10px] font-medium bg-slate-200/80 dark:bg-[#1a2338] text-slate-600 dark:text-slate-400 rounded-md"
              >
                Integrations & Catalog
              </span>
            </div>
            <p class="text-xs text-slate-500 dark:text-slate-400 mt-0.5 line-clamp-1">
              Deploy companion services or follow step-by-step guides to integrate with Hephaestus Control Panel.
            </p>
          </div>
        </div>

        <div class="flex items-center gap-2 shrink-0">
          <button
            @click="emit('close')"
            class="p-2 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer"
            title="Close (Esc)"
          >
            <X class="w-5 h-5" />
          </button>
        </div>
      </div>

      <!-- Main Body Area -->
      <div class="flex-1 flex flex-col min-h-0 overflow-hidden">
        <!-- VIEW 1: CATALOG GRID -->
        <template v-if="!selectedApp">
          <!-- Toolbar & Filter Area -->
          <div
            class="p-4 border-b border-slate-200 dark:border-[#1b2234] bg-white dark:bg-[#0f1422] shrink-0 space-y-3"
          >
            <div class="flex flex-col sm:flex-row items-center gap-3 justify-between">
              <!-- Search Input -->
              <div class="relative w-full sm:w-80">
                <Search
                  class="w-4 h-4 text-slate-400 dark:text-slate-500 absolute left-3 top-1/2 -translate-y-1/2"
                />
                <input
                  v-model="searchQuery"
                  type="text"
                  placeholder="Search applications..."
                  class="w-full pl-9 pr-3 py-1.5 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 placeholder-slate-400 focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                />
              </div>

              <!-- Docker Host Connection Status Indicator -->
              <div class="flex items-center gap-2 self-start sm:self-auto">
                <button
                  @click="goToConnections"
                  class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg border border-slate-200 dark:border-[#1e273d] bg-slate-50 dark:bg-[#131a29] hover:bg-slate-100 dark:hover:bg-[#182236] text-[11px] text-slate-600 dark:text-slate-400 transition cursor-pointer"
                  title="Manage Docker Connections"
                >
                  <Server class="w-3.5 h-3.5 text-slate-400 dark:text-slate-500" />
                  <span>
                    Docker Hosts:
                    <strong class="text-slate-800 dark:text-slate-200">
                      {{ dockerConnections.length }}
                    </strong>
                  </span>
                  <Link2 class="w-3 h-3 text-slate-400" />
                </button>
              </div>
            </div>

            <!-- Category Chips -->
            <div class="flex items-center gap-1.5 overflow-x-auto pb-1 scrollbar-none">
              <button
                v-for="cat in categories"
                :key="cat.id"
                @click="selectedCategory = cat.id"
                :class="[
                  'px-3 py-1 text-xs rounded-lg font-medium transition shrink-0 cursor-pointer',
                  selectedCategory === cat.id
                    ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-950 shadow-xs'
                    : 'bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-[#141b2a] dark:text-slate-400 dark:hover:bg-[#1c263c] dark:hover:text-slate-200',
                ]"
              >
                {{ cat.label }}
              </button>
            </div>
          </div>

          <!-- Apps Grid (Matching Layout in Screenshot 2 without FREE/ONE badges) -->
          <div class="flex-1 overflow-y-auto p-4 sm:p-5">
            <div
              v-if="filteredApps.length === 0"
              class="h-full flex flex-col items-center justify-center text-center p-8 space-y-3"
            >
              <div
                class="w-12 h-12 rounded-full bg-slate-100 dark:bg-[#151c2e] border border-slate-200 dark:border-[#1e283d] flex items-center justify-center text-slate-400"
              >
                <Boxes class="w-6 h-6" />
              </div>
              <p class="text-sm font-semibold text-slate-700 dark:text-slate-300">
                No applications found
              </p>
              <p class="text-xs text-slate-500 dark:text-slate-400">
                Try searching with a different term or clear the filter.
              </p>
            </div>

            <div
              v-else
              class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
            >
              <div
                v-for="app in filteredApps"
                :key="app.id"
                @click="openAppDetail(app)"
                class="group bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] hover:border-slate-300 dark:hover:border-slate-600 rounded-xl p-4 transition-all duration-200 hover:shadow-md cursor-pointer flex flex-col justify-between"
              >
                <!-- Top Row: Icon + Title (No badges per instructions) -->
                <div>
                  <div class="flex items-start gap-3">
                    <!-- App Icon / Emblem -->
                    <div
                      class="w-11 h-11 rounded-xl bg-slate-50 dark:bg-[#172036] border border-slate-200 dark:border-[#222e4a] flex items-center justify-center shrink-0 group-hover:scale-105 transition-transform"
                    >
                      <!-- Grafana -->
                      <Activity
                        v-if="app.id === 'grafana'"
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                      <!-- Data Prepper -->
                      <Sliders
                        v-else-if="app.id === 'data-prepper'"
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                      <!-- OpenTelemetry -->
                      <Cpu
                        v-else-if="app.id === 'opentelemetry'"
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                      <!-- Vaultwarden -->
                      <Shield
                        v-else-if="app.id === 'vaultwarden'"
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                      <!-- Prometheus -->
                      <Layers
                        v-else-if="app.id === 'prometheus'"
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                      <!-- OpenSearch -->
                      <Search
                        v-else-if="app.id === 'opensearch'"
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                      <Boxes
                        v-else
                        class="w-5 h-5 text-slate-600 dark:text-slate-300"
                      />
                    </div>

                    <!-- Title & Tag -->
                    <div class="min-w-0 flex-1">
                      <h3
                        class="text-sm font-bold text-slate-900 dark:text-white truncate group-hover:text-primary-600 dark:group-hover:text-primary-400 transition"
                      >
                        {{ app.name }}
                      </h3>
                      <!-- Category Breadcrumb Row matching Image 2 -->
                      <div class="flex items-center gap-1.5 text-[11px] text-slate-500 dark:text-slate-400 font-mono mt-0.5 truncate">
                        <FolderArchive class="w-3 h-3 text-slate-400 shrink-0" />
                        <span class="truncate">{{ app.category }}</span>
                      </div>
                    </div>
                  </div>

                  <!-- Description matching Image 2 -->
                  <p
                    class="text-xs text-slate-600 dark:text-slate-400 leading-relaxed line-clamp-3 mt-3"
                  >
                    {{ app.summary }}
                  </p>
                </div>

                <!-- Footer info / Action -->
                <div
                  class="pt-3 mt-3 border-t border-slate-100 dark:border-[#1a2236] flex items-center justify-between text-[11px] text-slate-500 dark:text-slate-400"
                >
                  <span class="font-mono text-[10px] text-slate-400 truncate max-w-[180px]">
                    {{ app.defaultImage }}
                  </span>
                  <span
                    class="text-slate-600 dark:text-slate-300 font-semibold group-hover:underline flex items-center gap-1"
                  >
                    Install & Guide →
                  </span>
                </div>
              </div>
            </div>
          </div>
        </template>

        <!-- VIEW 2: APP DETAIL & INSTALLATION -->
        <template v-else>
          <!-- Detail Navigation Bar -->
          <div
            class="px-5 py-3 border-b border-slate-200 dark:border-[#1b2234] bg-white dark:bg-[#0f1422] flex items-center justify-between shrink-0"
          >
            <button
              @click="backToCatalog"
              class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer"
            >
              <ArrowLeft class="w-4 h-4" />
              <span>Back to App Catalog</span>
            </button>

            <!-- Navigation Tabs -->
            <div class="flex items-center gap-1 bg-slate-100 dark:bg-[#141b2a] p-1 rounded-xl">
              <button
                @click="activeTab = 'docker'"
                :class="[
                  'px-3.5 py-1 text-xs font-semibold rounded-lg transition cursor-pointer flex items-center gap-1.5',
                  activeTab === 'docker'
                    ? 'bg-white dark:bg-[#1e283d] text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                ]"
              >
                <Boxes class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                <span>Install to Docker</span>
              </button>
              <button
                @click="activeTab = 'manual'"
                :class="[
                  'px-3.5 py-1 text-xs font-semibold rounded-lg transition cursor-pointer flex items-center gap-1.5',
                  activeTab === 'manual'
                    ? 'bg-white dark:bg-[#1e283d] text-slate-900 dark:text-white shadow-xs'
                    : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                ]"
              >
                <Terminal class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                <span>Manual Installation Guide</span>
              </button>
            </div>
          </div>

          <!-- App Overview Header Banner -->
          <div
            class="px-5 py-4 bg-slate-50/70 dark:bg-[#121827]/70 border-b border-slate-200 dark:border-[#1b2234] shrink-0"
          >
            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div class="flex items-center gap-3">
                <div
                  class="w-12 h-12 rounded-xl bg-white dark:bg-[#172036] border border-slate-200 dark:border-[#222e4a] flex items-center justify-center text-slate-600 dark:text-slate-300 shrink-0 shadow-xs"
                >
                  <Activity
                    v-if="selectedApp.id === 'grafana'"
                    class="w-6 h-6 text-slate-600 dark:text-slate-300"
                  />
                  <Sliders
                    v-else-if="selectedApp.id === 'data-prepper'"
                    class="w-6 h-6 text-slate-600 dark:text-slate-300"
                  />
                  <Cpu
                    v-else-if="selectedApp.id === 'opentelemetry'"
                    class="w-6 h-6 text-slate-600 dark:text-slate-300"
                  />
                  <Shield
                    v-else-if="selectedApp.id === 'vaultwarden'"
                    class="w-6 h-6 text-slate-600 dark:text-slate-300"
                  />
                  <Layers
                    v-else-if="selectedApp.id === 'prometheus'"
                    class="w-6 h-6 text-slate-600 dark:text-slate-300"
                  />
                  <Search
                    v-else-if="selectedApp.id === 'opensearch'"
                    class="w-6 h-6 text-slate-600 dark:text-slate-300"
                  />
                  <Boxes v-else class="w-6 h-6 text-slate-600 dark:text-slate-300" />
                </div>
                <div>
                  <h3 class="text-base font-bold text-slate-900 dark:text-white">
                    {{ selectedApp.name }}
                  </h3>
                  <div class="flex items-center gap-2 text-xs text-slate-500 dark:text-slate-400 font-mono mt-0.5">
                    <span>{{ selectedApp.category }}</span>
                  </div>
                </div>
              </div>

              <div class="flex items-center gap-2 shrink-0">
                <a
                  :href="selectedApp.officialDocs"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-semibold text-slate-600 dark:text-slate-300 hover:text-slate-900 dark:hover:text-white bg-white dark:bg-[#161f33] border border-slate-200 dark:border-[#222e4a] rounded-lg transition"
                >
                  <span>Official Documentation</span>
                  <ExternalLink class="w-3.5 h-3.5 text-slate-400" />
                </a>
              </div>
            </div>
          </div>

          <!-- Tab Content Scrollable Container -->
          <div class="flex-1 overflow-y-auto p-5">
            <!-- TAB 1: INSTALL TO DOCKER -->
            <div v-if="activeTab === 'docker'" class="max-w-3xl mx-auto space-y-5">
              <!-- Warning banner if NO Docker Connections exist -->
              <div
                v-if="dockerConnections.length === 0"
                class="p-4 sm:p-5 rounded-xl border border-amber-300 dark:border-amber-500/30 bg-amber-50 dark:bg-amber-500/10 space-y-3"
              >
                <div class="flex items-start gap-3">
                  <div
                    class="w-9 h-9 rounded-lg bg-amber-500/15 text-amber-600 dark:text-amber-400 flex items-center justify-center shrink-0 mt-0.5"
                  >
                    <AlertCircle class="w-5 h-5" />
                  </div>
                  <div class="space-y-1">
                    <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                      Docker Connection Required
                    </h4>
                    <p class="text-xs text-slate-600 dark:text-slate-300 leading-relaxed">
                      Hephaestus is running inside a Docker container. In order to deploy companion application containers, the Hephaestus host Docker daemon (or remote Docker host) must be configured in the
                      <strong>Connections</strong> menu first.
                    </p>
                  </div>
                </div>

                <div class="flex items-center gap-2 pt-1 pl-12">
                  <button
                    @click="goToConnections"
                    class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-slate-900 dark:bg-white text-white dark:text-slate-950 text-xs font-bold rounded-lg transition cursor-pointer shadow-xs"
                  >
                    <Link2 class="w-3.5 h-3.5" />
                    <span>Add Docker Connection in Connections Menu</span>
                  </button>
                  <button
                    @click="fetchDockerConnections"
                    :disabled="loadingConnections"
                    class="inline-flex items-center gap-1 px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white transition cursor-pointer"
                  >
                    <RefreshCw :class="['w-3.5 h-3.5', loadingConnections && 'animate-spin']" />
                    <span>Check Again</span>
                  </button>
                </div>
              </div>

              <!-- Deploy Success Banner -->
              <div
                v-if="deploySuccess"
                class="p-4 rounded-xl border border-emerald-500/30 bg-emerald-500/10 space-y-3 animate-in fade-in"
              >
                <div class="flex items-start gap-3">
                  <div
                    class="w-8 h-8 rounded-lg bg-emerald-500/20 text-emerald-500 flex items-center justify-center shrink-0"
                  >
                    <CheckCircle2 class="w-5 h-5" />
                  </div>
                  <div class="space-y-1 flex-1">
                    <h4 class="text-sm font-bold text-emerald-700 dark:text-emerald-300">
                      Application Container Deployed Successfully!
                    </h4>
                    <p class="text-xs text-slate-600 dark:text-slate-300">
                      Container <strong class="text-slate-800 dark:text-slate-100">{{ deployForm.name }}</strong> has been initialized and started on your Docker host.
                    </p>
                    <div class="text-[11px] font-mono text-slate-500 dark:text-slate-400 pt-0.5">
                      Container ID: {{ deployResultId }}
                    </div>
                  </div>
                </div>

                <div class="flex items-center gap-2 pt-1 pl-11">
                  <button
                    @click="goToContainers"
                    class="inline-flex items-center gap-1.5 px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-bold rounded-lg transition cursor-pointer shadow-xs"
                  >
                    <Boxes class="w-3.5 h-3.5" />
                    <span>View in Management Containers</span>
                  </button>
                  <button
                    @click="deploySuccess = false"
                    class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
                  >
                    Deploy Another
                  </button>
                </div>
              </div>

              <!-- Deploy Error Alert -->
              <div
                v-if="deployError"
                class="p-3.5 rounded-xl border border-rose-500/30 bg-rose-500/10 text-xs text-rose-700 dark:text-rose-300 flex items-start gap-2.5"
              >
                <AlertCircle class="w-4 h-4 shrink-0 mt-0.5 text-rose-500" />
                <div class="flex-1">
                  <strong>Deployment Error:</strong> {{ deployError }}
                </div>
              </div>

              <!-- Deployment Form Card -->
              <div
                class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-4"
              >
                <div class="flex items-center justify-between border-b border-slate-200 dark:border-[#1b2438] pb-3">
                  <div>
                    <h4 class="text-xs font-bold uppercase tracking-wider text-slate-700 dark:text-slate-300">
                      Docker Container Configuration
                    </h4>
                    <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                      Confirm or adjust configuration before deploying to your target Docker host.
                    </p>
                  </div>
                </div>

                <!-- Field 1: Target Docker Connection / Host -->
                <div class="space-y-1.5">
                  <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Target Docker Host Connection
                  </label>
                  <div class="relative">
                    <select
                      v-model="selectedConnectionId"
                      :disabled="dockerConnections.length === 0 || isDeploying"
                      class="w-full px-3 py-2 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition cursor-pointer disabled:opacity-50"
                    >
                      <option v-if="dockerConnections.length === 0" value="">
                        -- No Docker Connections Found --
                      </option>
                      <option
                        v-for="conn in dockerConnections"
                        :key="conn.id"
                        :value="conn.id"
                      >
                        {{ conn.name }} ({{ conn.host || 'local socket' }}){{ conn.isDefault ? ' [Default]' : '' }}
                      </option>
                    </select>
                  </div>
                  <p class="text-[10px] text-slate-500 dark:text-slate-400">
                    The Docker environment where this container will be spun up.
                  </p>
                </div>

                <!-- Field 2: Container Name & Docker Image -->
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div class="space-y-1.5">
                    <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Container Name
                    </label>
                    <input
                      v-model="deployForm.name"
                      type="text"
                      :disabled="isDeploying"
                      class="w-full px-3 py-2 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 font-mono focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                      placeholder="e.g. hephaestus-grafana"
                    />
                  </div>
                  <div class="space-y-1.5">
                    <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Docker Image
                    </label>
                    <input
                      v-model="deployForm.image"
                      type="text"
                      :disabled="isDeploying"
                      class="w-full px-3 py-2 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 font-mono focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                    />
                  </div>
                </div>

                <!-- Field 3: Port Mappings & Restart Policy -->
                <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div class="space-y-1.5">
                    <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Port Bindings (Host:Container, one per line)
                    </label>
                    <textarea
                      v-model="deployForm.portsText"
                      rows="2"
                      :disabled="isDeploying"
                      class="w-full px-3 py-1.5 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 font-mono focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                      placeholder="3000:3000"
                    ></textarea>
                  </div>
                  <div class="space-y-1.5">
                    <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                      Restart Policy
                    </label>
                    <select
                      v-model="deployForm.restartPolicy"
                      :disabled="isDeploying"
                      class="w-full px-3 py-2 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                    >
                      <option value="unless-stopped">Unless Stopped (Recommended)</option>
                      <option value="always">Always</option>
                      <option value="on-failure">On Failure</option>
                      <option value="no">No Restart</option>
                    </select>
                  </div>
                </div>

                <!-- Field 4: Volume Mounts -->
                <div class="space-y-1.5">
                  <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Volume Mounts (HostVolume:ContainerPath, one per line)
                  </label>
                  <textarea
                    v-model="deployForm.volumesText"
                    rows="2"
                    :disabled="isDeploying"
                    class="w-full px-3 py-1.5 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 font-mono focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                    placeholder="grafana-data:/var/lib/grafana"
                  ></textarea>
                </div>

                <!-- Field 5: Environment Variables -->
                <div class="space-y-1.5">
                  <label class="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Environment Variables (KEY=VALUE, one per line)
                  </label>
                  <textarea
                    v-model="deployForm.envVarsText"
                    rows="3"
                    :disabled="isDeploying"
                    class="w-full px-3 py-1.5 text-xs bg-slate-50 dark:bg-[#131a29] border border-slate-200 dark:border-[#1e273d] rounded-lg text-slate-900 dark:text-slate-100 font-mono focus:outline-none focus:border-slate-400 dark:focus:border-slate-500 transition"
                    placeholder="KEY=VALUE"
                  ></textarea>
                </div>

                <!-- Action Button -->
                <div class="pt-3 border-t border-slate-200 dark:border-[#1b2438] flex items-center justify-between">
                  <span class="text-[11px] text-slate-500 dark:text-slate-400">
                    Image will be pulled automatically if not present locally.
                  </span>

                  <button
                    @click="handleInstallToDocker"
                    :disabled="isDeploying"
                    class="inline-flex items-center gap-2 px-5 py-2.5 bg-slate-900 hover:bg-slate-800 dark:bg-white dark:hover:bg-slate-100 text-white dark:text-slate-950 font-bold text-xs rounded-xl transition cursor-pointer shadow-md disabled:opacity-50"
                  >
                    <RefreshCw v-if="isDeploying" class="w-4 h-4 animate-spin" />
                    <Play v-else class="w-4 h-4" />
                    <span>{{ isDeploying ? 'Deploying Container...' : 'Install to Docker' }}</span>
                  </button>
                </div>
              </div>
            </div>

            <!-- TAB 2: MANUAL INSTALLATION GUIDE -->
            <div v-else-if="activeTab === 'manual'" class="max-w-3xl mx-auto space-y-6">
              <!-- Method Switcher: Docker vs Systemd -->
              <div
                class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-200 dark:border-[#1b2234]"
              >
                <div>
                  <h4 class="text-xs font-bold text-slate-900 dark:text-white uppercase tracking-wider">
                    Deployment Runtime
                  </h4>
                  <p class="text-[11px] text-slate-500 dark:text-slate-400 mt-0.5">
                    Select your preferred runtime: containerized Docker environment or native Linux Systemd daemon.
                  </p>
                </div>

                <div class="flex items-center gap-1 bg-slate-100 dark:bg-[#141b2a] p-1 rounded-xl shrink-0">
                  <button
                    @click="manualMethod = 'docker'"
                    :class="[
                      'px-3 py-1.5 text-xs font-semibold rounded-lg transition cursor-pointer flex items-center gap-1.5',
                      manualMethod === 'docker'
                        ? 'bg-white dark:bg-[#1e283d] text-slate-900 dark:text-white shadow-xs'
                        : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                    ]"
                  >
                    <Boxes class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                    <span>Docker & Compose</span>
                  </button>
                  <button
                    @click="manualMethod = 'systemd'"
                    :class="[
                      'px-3 py-1.5 text-xs font-semibold rounded-lg transition cursor-pointer flex items-center gap-1.5',
                      manualMethod === 'systemd'
                        ? 'bg-white dark:bg-[#1e283d] text-slate-900 dark:text-white shadow-xs'
                        : 'text-slate-500 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white',
                    ]"
                  >
                    <Terminal class="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
                    <span>Systemd Service</span>
                  </button>
                </div>
              </div>

              <!-- DOCKER INSTALLATION FLOW -->
              <template v-if="manualMethod === 'docker'">
                <!-- Step 1: Requirements -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center gap-2">
                    <span
                      class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                    >
                      1
                    </span>
                    <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                      System & Port Requirements
                    </h4>
                  </div>
                  <p class="text-xs text-slate-600 dark:text-slate-400 leading-relaxed">
                    Make sure Docker engine is running on your host and the necessary ports are available on your network firewall:
                  </p>
                  <div class="p-3 rounded-lg bg-slate-50 dark:bg-[#151d2f] border border-slate-200 dark:border-[#1e273e] text-xs font-mono text-slate-700 dark:text-slate-300">
                    {{ selectedApp.manualInstall.portsDesc }}
                  </div>
                </div>

                <!-- Step 2: Run with Docker CLI -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span
                        class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                      >
                        2
                      </span>
                      <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                        Run via Docker CLI
                      </h4>
                    </div>
                    <button
                      @click="copyCode(selectedApp.manualInstall.runCmd, 'cli')"
                      class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer border border-slate-200 dark:border-[#222e4a]"
                    >
                      <Check v-if="copiedCodeType === 'cli'" class="w-3.5 h-3.5 text-emerald-500" />
                      <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
                      <span>{{ copiedCodeType === 'cli' ? 'Copied!' : 'Copy Command' }}</span>
                    </button>
                  </div>
                  <div class="relative">
                    <pre
                      class="p-4 rounded-xl bg-slate-900 text-slate-100 dark:bg-[#0a0e17] border border-slate-800 text-xs font-mono overflow-x-auto leading-relaxed"
                    >{{ selectedApp.manualInstall.runCmd }}</pre>
                  </div>
                </div>

                <!-- Step 3: Run with Docker Compose -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span
                        class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                      >
                        3
                      </span>
                      <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                        Or Run via Docker Compose (docker-compose.yml)
                      </h4>
                    </div>
                    <button
                      @click="copyCode(selectedApp.manualInstall.composeYaml, 'compose')"
                      class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer border border-slate-200 dark:border-[#222e4a]"
                    >
                      <Check v-if="copiedCodeType === 'compose'" class="w-3.5 h-3.5 text-emerald-500" />
                      <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
                      <span>{{ copiedCodeType === 'compose' ? 'Copied!' : 'Copy YAML' }}</span>
                    </button>
                  </div>
                  <div class="relative">
                    <pre
                      class="p-4 rounded-xl bg-slate-900 text-slate-100 dark:bg-[#0a0e17] border border-slate-800 text-xs font-mono overflow-x-auto leading-relaxed"
                    >{{ selectedApp.manualInstall.composeYaml }}</pre>
                  </div>
                </div>
              </template>

              <!-- SYSTEMD LINUX SERVICE FLOW -->
              <template v-else-if="manualMethod === 'systemd'">
                <!-- Step 1: Requirements -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center gap-2">
                    <span
                      class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                    >
                      1
                    </span>
                    <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                      System & Port Requirements
                    </h4>
                  </div>
                  <p class="text-xs text-slate-600 dark:text-slate-400 leading-relaxed">
                    Make sure systemd is available on your Linux distribution (Ubuntu, Debian, RHEL, Rocky Linux, or CentOS) and open necessary incoming firewall ports:
                  </p>
                  <div class="p-3 rounded-lg bg-slate-50 dark:bg-[#151d2f] border border-slate-200 dark:border-[#1e273e] text-xs font-mono text-slate-700 dark:text-slate-300">
                    {{ selectedApp.manualInstall.portsDesc }}
                  </div>
                </div>

                <!-- Step 2: Binary / Package Setup -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span
                        class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                      >
                        2
                      </span>
                      <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                        Install Packages & Prepare Directories
                      </h4>
                    </div>
                    <button
                      @click="copyCode(selectedApp.manualInstall.systemd.installCmd, 'sys-install')"
                      class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer border border-slate-200 dark:border-[#222e4a]"
                    >
                      <Check v-if="copiedCodeType === 'sys-install'" class="w-3.5 h-3.5 text-emerald-500" />
                      <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
                      <span>{{ copiedCodeType === 'sys-install' ? 'Copied!' : 'Copy Script' }}</span>
                    </button>
                  </div>
                  <div class="relative">
                    <pre
                      class="p-4 rounded-xl bg-slate-900 text-slate-100 dark:bg-[#0a0e17] border border-slate-800 text-xs font-mono overflow-x-auto leading-relaxed"
                    >{{ selectedApp.manualInstall.systemd.installCmd }}</pre>
                  </div>
                </div>

                <!-- Step 3: Create Systemd Unit -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span
                        class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                      >
                        3
                      </span>
                      <div>
                        <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                          Create Systemd Service Unit
                        </h4>
                        <p class="text-[11px] text-slate-500 dark:text-slate-400 font-mono mt-0.5">
                          /etc/systemd/system/{{ selectedApp.manualInstall.systemd.serviceName }}
                        </p>
                      </div>
                    </div>
                    <button
                      @click="copyCode(selectedApp.manualInstall.systemd.serviceUnit, 'sys-unit')"
                      class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer border border-slate-200 dark:border-[#222e4a]"
                    >
                      <Check v-if="copiedCodeType === 'sys-unit'" class="w-3.5 h-3.5 text-emerald-500" />
                      <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
                      <span>{{ copiedCodeType === 'sys-unit' ? 'Copied!' : 'Copy Unit File' }}</span>
                    </button>
                  </div>
                  <div class="relative">
                    <pre
                      class="p-4 rounded-xl bg-slate-900 text-slate-100 dark:bg-[#0a0e17] border border-slate-800 text-xs font-mono overflow-x-auto leading-relaxed"
                    >{{ selectedApp.manualInstall.systemd.serviceUnit }}</pre>
                  </div>
                </div>

                <!-- Step 4: Start & Enable Service -->
                <div
                  class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-3"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span
                        class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                      >
                        4
                      </span>
                      <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                        Enable & Start Service Daemon
                      </h4>
                    </div>
                    <button
                      @click="copyCode(selectedApp.manualInstall.systemd.manageCmd, 'sys-manage')"
                      class="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white hover:bg-slate-100 dark:hover:bg-[#182133] rounded-lg transition cursor-pointer border border-slate-200 dark:border-[#222e4a]"
                    >
                      <Check v-if="copiedCodeType === 'sys-manage'" class="w-3.5 h-3.5 text-emerald-500" />
                      <Copy v-else class="w-3.5 h-3.5 text-slate-400" />
                      <span>{{ copiedCodeType === 'sys-manage' ? 'Copied!' : 'Copy Commands' }}</span>
                    </button>
                  </div>
                  <div class="relative">
                    <pre
                      class="p-4 rounded-xl bg-slate-900 text-slate-100 dark:bg-[#0a0e17] border border-slate-800 text-xs font-mono overflow-x-auto leading-relaxed"
                    >{{ selectedApp.manualInstall.systemd.manageCmd }}</pre>
                  </div>
                </div>
              </template>

              <!-- Final Step: Connecting to Hephaestus Control Panel -->
              <div
                class="bg-white dark:bg-[#111728] border border-slate-200 dark:border-[#1f283d] rounded-xl p-5 space-y-4"
              >
                <div class="flex items-center gap-2">
                  <span
                    class="w-6 h-6 rounded-full bg-slate-100 dark:bg-[#1c263c] text-slate-700 dark:text-slate-300 text-xs font-bold flex items-center justify-center"
                  >
                    {{ manualMethod === 'docker' ? '4' : '5' }}
                  </span>
                  <h4 class="text-sm font-bold text-slate-900 dark:text-white">
                    Connecting to Hephaestus Control Panel
                  </h4>
                </div>
                <div class="space-y-2.5 pl-8">
                  <div
                    v-for="(step, idx) in selectedApp.manualInstall.hcpSteps"
                    :key="idx"
                    class="flex items-start gap-2.5 text-xs text-slate-600 dark:text-slate-300 leading-relaxed"
                  >
                    <span class="font-bold text-slate-400 shrink-0">{{ idx + 1 }}.</span>
                    <span>{{ step }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <!-- POPUP MODAL: No Docker Connection (per user requirement) -->
    <div
      v-if="showNoConnectionModal"
      class="fixed inset-0 z-60 flex items-center justify-center p-4 bg-slate-900/60 dark:bg-black/80 backdrop-blur-sm animate-in fade-in"
    >
      <div
        class="bg-white dark:bg-[#111624] border border-slate-200 dark:border-[#1f283d] rounded-2xl w-full max-w-sm shadow-2xl p-5 space-y-4 text-center"
      >
        <div
          class="w-12 h-12 rounded-full bg-amber-500/10 text-amber-500 flex items-center justify-center mx-auto"
        >
          <Server class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h3 class="text-sm font-bold text-slate-900 dark:text-white">
            Docker Connection Required
          </h3>
          <p class="text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
            Hephaestus is running in Docker and requires a Docker Connection in the Connections menu to deploy containers. Please register your Hephaestus host or Docker engine first.
          </p>
        </div>
        <div class="flex items-center justify-center gap-2 pt-2">
          <button
            @click="showNoConnectionModal = false"
            class="px-3 py-1.5 text-xs text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-white cursor-pointer"
          >
            Cancel
          </button>
          <button
            @click="goToConnections"
            class="px-4 py-1.5 bg-slate-900 dark:bg-white text-white dark:text-slate-950 rounded-lg text-xs font-bold transition cursor-pointer"
          >
            Add Connection Now
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
