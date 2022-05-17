import './App.css';
import { useState, useEffect } from "react";
import moment from 'moment'



function App() {
  const [services, setServices] = useState([]);
  const [updateInfo, setUpdateInfo] = useState("Loading");

  const fetchServices = () => {
    fetch('./status')
      .then(response => response.json())
      .then(response => {
        response.sort((a, b) => {
          let fa = a.name.toLowerCase(),
              fb = b.name.toLowerCase();
          if (fa < fb) {
              return -1;
          }
          if (fa > fb) {
              return 1;
          }
          return 0;
        })
        setServices(response)
        setUpdateInfo("Last updated: "+moment(new Date()).format("LTS"))
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
      <div id="wrapper">
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
        <p className='has-text-right has-text-weight-light	'>
            {updateInfo}
        </p>
    </div>
  );
}

export default App;
