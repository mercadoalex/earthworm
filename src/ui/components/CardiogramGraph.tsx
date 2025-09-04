// CardiogramGraph.tsx
import React, { useEffect, useState } from 'react';
import { Line } from 'react-chartjs-2'; // Importing Line chart component from Chart.js

// Define the structure of the heartbeat data
interface HeartbeatData {
    timestamp: number; // Time of the heartbeat
    value: number; // Heartbeat value
}

// CardiogramGraph component to visualize heartbeat data
const CardiogramGraph: React.FC = () => {
    const [data, setData] = useState<HeartbeatData[]>([]); // State to hold heartbeat data
    const [loading, setLoading] = useState<boolean>(true); // State to manage loading state

    // Function to fetch heartbeat data from the server
    const fetchHeartbeatData = async () => {
        try {
            const response = await fetch('/api/heartbeat'); // Fetching data from the server endpoint
            const result = await response.json(); // Parsing the JSON response
            setData(result); // Updating state with the fetched data
            setLoading(false); // Setting loading to false after data is fetched
        } catch (error) {
            console.error('Error fetching heartbeat data:', error); // Logging any errors
            setLoading(false); // Setting loading to false in case of error
        }
    };

    // Effect to fetch data when the component mounts
    useEffect(() => {
        fetchHeartbeatData(); // Calling the fetch function
        const interval = setInterval(fetchHeartbeatData, 5000); // Setting up an interval to fetch data every 5 seconds
        return () => clearInterval(interval); // Cleanup interval on component unmount
    }, []);

    // Preparing data for the chart
    const chartData = {
        labels: data.map(d => new Date(d.timestamp).toLocaleTimeString()), // Formatting timestamps for labels
        datasets: [
            {
                label: 'Heartbeat', // Label for the dataset
                data: data.map(d => d.value), // Heartbeat values for the dataset
                borderColor: 'rgba(75,192,192,1)', // Line color
                backgroundColor: 'rgba(75,192,192,0.2)', // Area color under the line
                fill: true, // Fill the area under the line
            },
        ],
    };

    // Render loading state or the chart
    return (
        <div>
            {loading ? ( // Conditional rendering based on loading state
                <p>Loading heartbeat data...</p> // Loading message
            ) : (
                <Line data={chartData} /> // Rendering the Line chart with the prepared data
            )}
        </div>
    );
};

export default CardiogramGraph; // Exporting the component for use in other parts of the application