// --- Imports ---
import { easingEffects } from 'chart.js/helpers';
import leasesByNamespace from './mocking_data/leases.json';


// --- Dynamically extract namespaces and assign green color to all ---
const namespaces = Object.keys(leasesByNamespace);
console.log('Total namespaces:', namespaces.length);
const namespaceColors = namespaces.map(() => 'rgb(7, 179, 90)');
console.log('Namespace colors:', namespaceColors);

// --- Build datasets dynamically from leases.json ---
// Each dataset represents a namespace. We swap x/y so:
//   x: timestamp (Date object, for time axis)
//   y: heartbeat number (for vertical axis)
const datasets = namespaces.map((ns, idx) => ({
  label: ns,
  borderColor: namespaceColors[idx], // All lines green
  borderWidth: 1,
  radius: 0,
  data: leasesByNamespace[ns].map((point, i) => {
    // Add very small spikes for cardiogram effect
    let spike = 0;
    if (i % 8 === 0) spike = 0.05;      // extremely small positive spike
    else if (i % 5 === 0) spike = -0.03; // extremely small negative spike
    return {
      x: new Date(point.y),
      y: point.x + spike
    };
  }),
  segment: { borderColor: getSegmentColor }
}));

datasets.forEach(ds => {
  console.log(`Namespace: ${ds.label}, Color: ${ds.borderColor}, Data points: ${ds.data.length}`);
});

// --- Heartbeat interval and animation duration calculation ---
// Heartbeat interval in ms (parametizable)
const HEARTBEAT_INTERVAL = 10000; // 10 seconds by default

// Number of heartbeats (from your data)
const heartbeatCount = datasets[0]?.data?.length || 22; // fallback to 22 if no data

// Calculate total duration for animation
const totalDuration = HEARTBEAT_INTERVAL * heartbeatCount; // ms

const demoLength = heartbeatCount;

// --- Animation configuration ---
let easing = easingEffects.easeOutQuad; // Default easing function

const duration = (ctx) => easing(ctx.index / demoLength) * totalDuration / demoLength;
const delay = (ctx) => easing(ctx.index / demoLength) * totalDuration;

// --- Previous Y value for animation (with guard) ---
const previousY = (ctx) => {
  const meta = ctx.chart.getDatasetMeta(ctx.datasetIndex);
  const prevData = meta.data[ctx.index - 1];
  if (ctx.index === 0 || !prevData || typeof prevData.getProps !== 'function') {
    return ctx.chart.scales.y.getPixelForValue(100);
  }
  return prevData.getProps(['y'], true).y;
};

// --- Animation object for Chart.js ---
const animation = {
  x: {
    type: 'number',
    easing: 'linear',
    duration: duration,
    from: NaN,
    delay(ctx) {
      if (ctx.type !== 'data' || ctx.xStarted) return 0;
      ctx.xStarted = true;
      return delay(ctx);
    }
  },
  y: {
    type: 'number',
    easing: 'linear',
    duration: duration,
    from: previousY,
    delay(ctx) {
      if (ctx.type !== 'data' || ctx.yStarted) return 0;
      ctx.yStarted = true;
      return delay(ctx);
    }
  }
};

// --- Segment color logic based on heartbeat intervals ---
// - Uses green for normal segments
// - Uses yellow/red for warning/critical intervals only
function getSegmentColor(ctx) {
  // Fallback for missing data
  if (!ctx.dataset || !ctx.dataset.data || ctx.dataset.data.length === 0) {
    // Only log once for debugging
    if (!getSegmentColor.warned) {
      console.warn('Missing dataset/data in getSegmentColor');
      getSegmentColor.warned = true;
    }
    return 'rgb(7, 179, 90)'; // Always green for missing data
  }
  const data = ctx.dataset.data;
  const i = ctx.p0?.index;

  // First point or missing index: use green
  if (typeof i !== 'number' || i === 0) {
    return 'rgb(7, 179, 90)';
  }

  // Invalid data: use green
  if (
    !data[i - 1] ||
    !data[i] ||
    !(data[i - 1].x instanceof Date) ||
    !(data[i].x instanceof Date)
  ) {
    return Utils.CHART_COLORS.green;
  }

  // Calculate interval in seconds using timestamps
  const prev = data[i - 1].x.getTime();
  const curr = data[i].x.getTime();
  const interval = (curr - prev) / 1000; // seconds

  // Previous two intervals for recovery logic
  let prevIntervals = [];
  if (i > 1 && data[i - 2].x instanceof Date)
    prevIntervals.push((data[i - 1].x.getTime() - data[i - 2].x.getTime()) / 1000);
  if (i > 2 && data[i - 3].x instanceof Date)
    prevIntervals.push((data[i - 2].x.getTime() - data[i - 3].x.getTime()) / 1000);

  // If interval > 40s, use yellow/red for warnings/critical only
  if (interval > 40) {
    const recentIntervals = [interval, ...prevIntervals].filter(v => v > 40);
    if (recentIntervals.length >= 2) {
      return 'rgb(255, 99, 132)'; // Critical
    }
    return 'rgb(255, 205, 86)'; // Warning
  }

  // For normal segments, always use green
  return 'rgb(7, 179, 90)';
}

// --- Chart.js configuration object ---
// X axis: time (Date object)
// Y axis: heartbeat number
const config = {
  type: 'line',
  data: {
    datasets: datasets
  },
  options: {
    animation,
    interaction: { intersect: false },
    plugins: {
      legend: false,
      title: {
        display: true,
        text: () => {
          let formattedDay = '';
          if (datasets.length > 0 && datasets[0].data.length > 0) {
            const date = datasets[0].data[0].x;
            formattedDay = date.toLocaleDateString('en-US', {
              weekday: 'long',
              year: 'numeric',
              month: 'long',
              day: 'numeric'
            });
          }
          return `${formattedDay} | ${easing.name}`;
        }
      },
      tooltip: {
        callbacks: {
          label: function(context) {
            const namespace = context.dataset.label || '';
            const heartbeat = context.parsed.y;
            const time = new Date(context.parsed.x).toLocaleTimeString();
            return `${namespace} | heartbeat: ${heartbeat} | time: ${time}`;
          }
        }
      }
    },
    scales: {
      x: {
        type: 'time', // X axis is now time
        time: {
          unit: 'minute',
          tooltipFormat: 'HH:mm:ss'
        },
        title: {
          display: true,
          text: 'Time (UTC)' // Show UTC label
        }
      },
      y: {
        type: 'linear',
        title: {
          display: true,
          text: 'Heartbeat'
        },
        ticks: {
          stepSize: 1
        }
      }
    }
  }
};

// --- Animation restart logic for easing changes ---
function restartAnims(chart) {
  chart.stop();
  datasets.forEach((ds, idx) => {
    const meta = chart.getDatasetMeta(idx);
    for (let i = 0; i < demoLength; i++) {
      const ctx = meta.controller.getContext(i);
      ctx.xStarted = ctx.yStarted = false;
    }
  });
  chart.update();
}

// --- Easing actions for UI ---
const actions = [
  {
    name: 'easeOutQuad',
    handler(chart) {
      easing = easingEffects.easeOutQuad;
      restartAnims(chart);
    }
  },
  {
    name: 'easeOutCubic',
    handler(chart) {
      easing = easingEffects.easeOutCubic;
      restartAnims(chart);
    }
  },
  {
    name: 'easeOutQuart',
    handler(chart) {
      easing = easingEffects.easeOutQuart;
      restartAnims(chart);
    }
  },
  {
    name: 'easeOutQuint',
    handler(chart) {
      easing = easingEffects.easeOutQuint;
      restartAnims(chart);
    }
  },
  {
    name: 'easeInQuad',
    handler(chart) {
      easing = easingEffects.easeInQuad;
      restartAnims(chart);
    }
  },
  {
    name: 'easeInCubic',
    handler(chart) {
      easing = easingEffects.easeInCubic;
      restartAnims(chart);
    }
  },
  {
    name: 'easeInQuart',
    handler(chart) {
      easing = easingEffects.easeInQuart;
      restartAnims(chart);
    }
  },
  {
    name: 'easeInQuint',
    handler(chart) {
      easing = easingEffects.easeInQuint;
      restartAnims(chart);
    }
  },
];

// --- Recharts utility for colors and data transformation ---

// --- Utility to transform leases data for Recharts ---
// Returns: { chartData, namespaces, namespaceColors }
function getRechartsData(currentHeartbeat) {
  // chartData: array of objects, each with keys for each namespace
  const chartData = [];
  const maxPoints = currentHeartbeat + 1;
  for (let i = 0; i < maxPoints; i++) {
    const point = { index: i };
    namespaces.forEach(ns => {
      if (leasesByNamespace[ns][i]) {
        // Use .x for value, .y for time if needed
        point[ns] = leasesByNamespace[ns][i].x;
        // Optionally, you can add time: point[ns + '_time'] = leasesByNamespace[ns][i].y;
      }
    });
    chartData.push(point);
  }
  return { chartData, namespaces, namespaceColors };
}

// --- Heartbeat animation logic ---
function handleHeartbeatAnimation(currentHeartbeat, totalHeartbeats, setCurrentHeartbeat) {
  if (currentHeartbeat < totalHeartbeats - 1) {
    const timer = setTimeout(() => {
      setCurrentHeartbeat(hb => hb + 1);
    }, 500); // Slower animation: 500ms per heartbeat
    return () => clearTimeout(timer);
  }
}

// --- Export config and actions for use in React component ---
export { config, actions, namespaces, namespaceColors, HEARTBEAT_INTERVAL, getRechartsData, handleHeartbeatAnimation };

// --- End of file ---