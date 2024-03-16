import "./App.css";
import { useState, useEffect } from "react";
import moment from "moment";
import { parse } from "query-string";

function App() {
  const [namespaces, setNamespaces] = useState([]);
  const [updateInfo, setUpdateInfo] = useState("Loading...");

  const fetchServices = () => {
    fetch("./status")
      .then((response) => response.json())
      .then((response) => {

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
        });
        let namespaces = {};
        response.forEach((service) => {
          if (!namespaces[service.namespace]) {
            namespaces[service.namespace] = [];
          }
          namespaces[service.namespace].push(service);
        });
        setNamespaces(Object.keys(namespaces).sort().reduce(
          (obj, key) => {
            obj[key] = namespaces[key];
            return obj;
          },
          {}
        ));
        setUpdateInfo("Last updated: " + moment(new Date()).format("LTS"));
      });
  };

  const refresh = parse(window.location.search).refresh || 5;
  const showHeader = parse(window.location.search).mode !== "tv";

  useEffect(() => {
    fetchServices();
    const interval = setInterval(fetchServices, refresh * 1000);
    return () => {
      clearInterval(interval);
    };
  }, [refresh]);

  return (
    <div className="App p-5">
      <div id="wrapper">
        {showHeader ? (
          <section className="hero">
            <div className="hero-body">
              <p className="title">k8status</p>
            </div>
          </section>
        ) : (
          ""
        )}
        <div>
          {Object.keys(namespaces).map((namespace, index) => {
            return (
              <div key={index}>
                {Object.keys(namespaces).length > 1 ? <h1 className="title is-4 mt-4">{namespace}</h1> : ""}
                <ServiceBlocks services={namespaces[namespace]} />
              </div>
            );
          })}
        </div>
      </div>
      <footer className="has-text-right has-text-weight-light	is-family-monospace is-size-7">
        {updateInfo}
      </footer>
    </div>
  );
}

function ServiceBlocks({ services }) {
  return (
    <div className="tile is-ancestor is-flex-wrap-wrap">
      {services.map((service, index) => {
        return (
          <div key={index} className="tile is-parent is-3">
            <article
              className={`tile is-child box notification ${getColor(
                service.status
              )}`}
            >
              <p className="title">{service.name}</p>
              {service.description ? (
                <p className="description has-text-dark">
                  {service.description}
                </p>
              ) : (
                ""
              )}
            </article>
          </div>
        );
      })}
    </div>
  );
}

function getColor(status) {
  switch (status) {
    case "ok":
      return "is-primary";
    case "down":
      return "is-danger";
    default:
      return "is-warning";
  }
}

export default App;
