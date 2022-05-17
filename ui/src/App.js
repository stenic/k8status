import './App.css';
import { useState, useEffect } from "react";
import moment from 'moment';
import { parse } from 'query-string';



function App() {
  const [services, setServices] = useState([]);
  const [updateInfo, setUpdateInfo] = useState("Loading...");

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

  const refresh = parse(window.location.search).refresh || 5;
  const showHeader = parse(window.location.search).mode !== "tv";

  useEffect(() => {
    fetchServices()
    const interval = setInterval(fetchServices, refresh * 1000);
    return () => {
      clearInterval(interval);
    };
  }, [refresh])
  
  return (
    <div className="App p-5">
      <div id="wrapper">
        {showHeader ? (
          <section class="hero">
            <div class="hero-body">
              <p class="title">
                k8status
              </p>
            </div>
          </section>
        ) : ""}
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
