import React, { useEffect, useState } from 'react'; // Importing React and necessary hooks
import CardiogramGraph from './components/CardiogramGraph'; // Importing the CardiogramGraph component

// Main App component
const App: React.FC = () => {
    // State to hold heartbeat data
    const [heartbeatData, setHeartbeatData] = useState<any[]>([]); // Initializing state for heartbeat data

    // Effect to fetch heartbeat data from the server
    useEffect(() => {
        const fetchHeartbeatData = async () => {
            try {
                const response = await fetch('/api/heartbeat'); // Fetching data from the server endpoint
                const data = await response.json(); // Parsing the JSON response
                setHeartbeatData(data); // Updating state with the fetched data
            } catch (error) {
                console.error('Error fetching heartbeat data:', error); // Logging any errors that occur during fetch
            }
        };

        fetchHeartbeatData(); // Calling the fetch function

        const interval = setInterval(fetchHeartbeatData, 5000); // Setting up an interval to fetch data every 5 seconds

        return () => clearInterval(interval); // Cleanup function to clear the interval on component unmount
    }, []); // Empty dependency array to run effect only on mount

    // Rendering the main application structure
    return (
        <div>
            <h1>Earthworm Heartbeat Monitor</h1> {/* Main title of the application */}
            <CardiogramGraph data={heartbeatData} /> {/* Rendering the CardiogramGraph component with heartbeat data */}
        </div>
    );
};

export default App; // Exporting the App component for use in other parts of the application