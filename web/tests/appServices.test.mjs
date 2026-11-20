import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('../src/lib/appServices.ts', import.meta.url), 'utf8');
const compiled = ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ES2022 } }).outputText;
const { appServices, canReadAppServices, isAppServicesPath } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`);

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
