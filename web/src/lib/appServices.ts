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

// Filter permissions before calling this function so counts never include hidden services.
export function getServicePage<T>(items: readonly T[], requestedPage: number, requestedSize: number) {
  const size = Number.isFinite(requestedSize) && requestedSize > 0 ? Math.max(1, Math.floor(requestedSize)) : 6;
  const totalPages = Math.max(1, Math.ceil(items.length / size));
  const page = Number.isFinite(requestedPage) ? Math.min(totalPages, Math.max(1, Math.floor(requestedPage))) : 1;
  const start = items.length ? (page - 1) * size + 1 : 0;
  const end = Math.min(page * size, items.length);
  return { page, totalPages, start, end, items: items.slice((page - 1) * size, page * size) };
}
