import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('../src/lib/appServices.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText;
const { appServices, canReadAppServices, isAppServicesPath, getServicePage } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);

test('catalog keeps the four services and their existing routes and permissions', () => {
  assert.deepEqual(appServices.map(({ name, feature, path }) => [name, feature, path]), [
    ['OpenSearch', 'opensearch', '/opensearch-cluster'],
    ['Data Prepper', 'dataprepper_config', '/dataprepper-config'],
    ['Prometheus', 'prometheus_config', '/prometheus-config'],
    ['OpenTelemetry', 'opentelemetry_config', '/opentelemetry-config'],
  ]);
  assert.equal(appServices.find(service => service.id === 'opentelemetry').visualPath, '/opentelemetry-visual');
  assert.equal(appServices.find(service => service.id === 'opensearch').newTab, true);
});

test('any one service permission grants catalog access, without requiring OpenSearch', () => {
  for (const service of appServices) {
    const can = feature => feature === service.feature;
    assert.equal(canReadAppServices(can), true);
    assert.deepEqual(appServices.filter(item => can(item.feature)), [service]);
  }
  assert.equal(canReadAppServices(() => false), false);
  assert.equal(canReadAppServices(feature => feature === 'remote_servers'), false);
});

test('application routes belong to App Services, while SSH and inventory do not', () => {
  for (const path of ['/app-services', '/opentelemetry-visual', ...appServices.map(service => service.path)]) {
    assert.equal(isAppServicesPath(path), true, path);
  }
  for (const path of ['/remote-host', '/remote-server', '/inventory-server', '/infrastructure/containers', '/']) {
    assert.equal(isAppServicesPath(path), false, path);
  }
});

test('catalog pagination slices pages with accurate totals and clamps page boundaries', () => {
  const first = getServicePage(appServices, 1, 3);
  assert.deepEqual(first.items, appServices.slice(0, 3));
  assert.deepEqual([first.page, first.totalPages, first.start, first.end], [1, 2, 1, 3]);
  const last = getServicePage(appServices, 99, 3);
  assert.deepEqual(last.items, [appServices[3]]);
  assert.deepEqual([last.page, last.totalPages, last.start, last.end], [2, 2, 4, 4]);
  assert.equal(getServicePage(appServices, -1, 3).page, 1);
  assert.equal(getServicePage(appServices, 2, 6).page, 1);
});

test('empty and permission-filtered catalogs have no misleading counts or hidden entries', () => {
  assert.deepEqual(getServicePage([], 5, 3), { page: 1, totalPages: 1, start: 0, end: 0, items: [] });
  const visible = appServices.filter(service => service.feature === 'prometheus_config');
  const result = getServicePage(visible, 2, 3);
  assert.deepEqual(result.items, visible);
  assert.deepEqual([result.page, result.totalPages, result.start, result.end], [1, 1, 1, 1]);
  assert.equal(getServicePage(appServices, NaN, 0).items.length, 4);
});
