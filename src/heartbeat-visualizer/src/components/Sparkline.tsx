import React from 'react';
import { LineChart, Line, YAxis } from 'recharts';
import { getSparklineColor } from '../utils/sparklineUtils';

// Feature: realistic-data-and-visualizations

interface SparklineProps {
  intervals: number[];
  width?: number;
  height?: number;
}

interface SparklinePoint {
  index: number;
  value: number;
  color: string;
}

const Sparkline: React.FC<SparklineProps> = ({ intervals, width = 120, height = 30 }) => {
  if (!intervals || intervals.length === 0) {
    return <div style={{ width, height }} aria-label="No sparkline data" />;
  }

  const data: SparklinePoint[] = intervals.map((v, i) => ({
    index: i,
    value: v,
    color: getSparklineColor(v),
  }));

  return (
    <div aria-label="Sparkline" data-testid="sparkline" style={{ display: 'inline-block' }}>
      <LineChart width={width} height={height} data={data}>
        <YAxis hide domain={['dataMin', 'dataMax']} />
        <Line
          type="monotone"
          dataKey="value"
          stroke="#aaa"
          strokeWidth={1.5}
          dot={false}
          isAnimationActive={false}
        />
      </LineChart>
    </div>
  );
};

export default Sparkline;
