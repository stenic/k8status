
module.exports = {
  webpack: function(config, env) {
    return config;
  },

  jest: config => {
    return config;
  },

  devServer: configFunction => (proxy, allowedHost) => {
    const config = configFunction(proxy, allowedHost);
    config.proxy = {
      "/status": {
          //  target: 'http://localhost:3000',
           router: () => process.env.PROXY_OVERWRITE || proxy[0].target,
           logLevel: 'debug' /*optional*/
      }
   }
    return config;
  },

  paths: (paths, env) => {
    return paths;
  }
};