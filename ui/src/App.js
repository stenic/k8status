import './App.css';
import { useState, useEffect } from "react";


function App() {
  const [services, setServices] = useState([]);

  const fetchServices = () => {
    fetch('./status')
      .then(response => response.json())
      .then(response => {
        setServices(response)
      })
  }

  useEffect(() => {
    const interval = setInterval(fetchServices, 1000);
    return () => {
      clearInterval(interval);
    };
  }, [])
  
  return (
    <div className="App p-5">
      <section class="hero">
        <div class="hero-body">
          <p class="title">
            k8status
          </p>
        </div>
      </section>
        <div class="tile is-ancestor is-flex-wrap-wrap">
          {services.map((service, index) => {
            return (
              <div key={index} class="tile is-parent is-3">
                <article className={`tile is-child box notification ${service.ready ? 'is-primary' : 'is-danger'}`}>
                  <p class="title">{service.name}</p>
                </article>
              </div>
            )
          })}
        </div>
    </div>
  );
}

export default App;
