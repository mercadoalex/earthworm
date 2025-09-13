const leases = require('./leases.json'); // Adjust path if needed

function checkDates(namespace) {
  const arr = leases[namespace];
  if (!arr) return console.log('Namespace not found');
  const dates = arr.map(obj => new Date(obj.y));
  const uniqueDates = new Set(dates.map(d => `${d.getFullYear()}-${d.getMonth()+1}-${d.getDate()}`));
  console.log(`Unique dates for ${namespace}:`, Array.from(uniqueDates));
}

checkDates('kube-node-lease'); // Change to your namespace of interest
checkDates('kube-system');