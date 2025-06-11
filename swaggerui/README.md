The Swagger UI in the `html` folder is taken from [https://github.com/swagger-api/swagger-ui](https://github.com/swagger-api/swagger-ui).

Update Swagger UI version:
- Download the latest [release](https://github.com/swagger-api/swagger-ui/releases)
- Unpack and copy the `dist` folder into `html`
- Replace `url` in file `swagger-initializer.js` with the correct path: `./swagger.json`