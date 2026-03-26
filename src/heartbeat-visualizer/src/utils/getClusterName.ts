import fs from 'fs';
import yaml from 'js-yaml';
import path from 'path';

interface ClusterConfig {
  clusters?: Array<{ name?: string }>;
}

export function getClusterName(): string {
  const configPath = path.resolve(__dirname, '..', 'config.yaml');
  const config = yaml.load(fs.readFileSync(configPath, 'utf8')) as ClusterConfig;
  return config?.clusters?.[0]?.name || 'Unknown Cluster';
}
