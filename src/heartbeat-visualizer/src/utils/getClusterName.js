import fs from 'fs';
import yaml from 'js-yaml';

export function getClusterName() {
  const configPath = __dirname + '/../config.yaml';
  const config = yaml.load(fs.readFileSync(configPath, 'utf8'));
  return config.clusters?.[0]?.name || 'Unknown Cluster';
}