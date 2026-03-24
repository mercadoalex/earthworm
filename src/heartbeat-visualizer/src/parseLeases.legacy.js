// This script parses leases.yaml, splits Lease objects by namespace,
// converts them into [{x, y}, ...] format, and saves the result as leases+timestamp.json for frontend use.

// Import required modules
const fs = require('fs');         // For reading files and writing output
const yaml = require('js-yaml');  // For parsing YAML
const path = require('path');

// Resolve the path to leases.yaml
const leasesPath = path.resolve(__dirname, '../../leases.yaml');

console.log('Looking for leases.yaml at:', leasesPath);
if (!fs.existsSync(leasesPath)) {
  throw new Error('leases.yaml not found at ' + leasesPath);
}

// Load and parse the leases.yaml file
const leases = yaml.load(fs.readFileSync(leasesPath, 'utf8'));

// Prepare an object to hold Lease data grouped by namespace
const leasesByNamespace = {};

// Log total items
console.log('Total items in YAML:', leases.items.length);

// Group items by namespace
leases.items.forEach(item => {
  const ns = item.metadata.namespace;
  if (!leasesByNamespace[ns]) leasesByNamespace[ns] = [];
  leasesByNamespace[ns].push(item);
});

// Log grouped counts
Object.keys(leasesByNamespace).forEach(ns => {
  console.log(`Namespace: ${ns}, Items: ${leasesByNamespace[ns].length}`);
});

// Convert each namespace's Lease array to [{x, y}, ...] format
Object.keys(leasesByNamespace).forEach(ns => {
  leasesByNamespace[ns] = leasesByNamespace[ns].map((lease, idx) => ({
    x: idx,
    y: new Date(lease.spec.renewTime).getTime()
  }));
  // Log converted points
  console.log(`Namespace: ${ns}, Converted points:`, leasesByNamespace[ns]);
});

// Generate timestamp in format YYYYMMDDTHHmmss
const now = new Date();
const pad = n => n.toString().padStart(2, '0');
const timestamp = 
  now.getFullYear() +
  pad(now.getMonth() + 1) +
  pad(now.getDate()) +
  'T' +
  pad(now.getHours()) +
  pad(now.getMinutes()) +
  pad(now.getSeconds());

// Write to leases+timestamp.json
const outFile = `${__dirname}/leases${timestamp}.json`;
fs.writeFileSync(outFile, JSON.stringify(leasesByNamespace, null, 2));
console.log('leases JSON written to', outFile);

console.log('leases JSON has been created successfully.');