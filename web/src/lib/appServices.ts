export const appServices = [
  { id: 'opensearch', name: 'OpenSearch', description: 'Explore cluster health, indices, and search activity.', feature: 'opensearch', path: '/opensearch-cluster', newTab: true },
  { id: 'dataprepper', name: 'Data Prepper', description: 'Configure and manage data ingestion pipelines.', feature: 'dataprepper_config', path: '/dataprepper-config', newTab: false },
  { id: 'prometheus', name: 'Prometheus', description: 'Manage scrape targets and metrics configuration.', feature: 'prometheus_config', path: '/prometheus-config', newTab: false },
  { id: 'opentelemetry', name: 'OpenTelemetry', description: 'Configure collectors, receivers, processors, and exporters.', feature: 'opentelemetry_config', path: '/opentelemetry-config', newTab: false, visualPath: '/opentelemetry-visual' },
] as const;

export function canReadAppServices(canRead: (feature: string) => boolean): boolean {
  return appServices.some(service => canRead(service.feature));
}

export function isAppServicesPath(path: string): boolean {
  return path === '/app-services' || appServices.some(service => path === service.path) || path === '/opentelemetry-visual';
}
