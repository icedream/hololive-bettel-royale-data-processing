import { useState, useEffect } from 'react';
import logo from './logo.svg';
import './App.scss';

import { getDB, importDB, initDB } from './database';
import { Database } from '@stephen/sql.js';
import MainPage from './components/pages/main';
import AppNavbar from './components/navigation/Navbar';
import { Container, Section } from 'react-bulma-components';

function App() {
  const [db, setDB] = useState<Database | null>(null);
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    async function fetchDB() {
      // sql.js needs to fetch its wasm file, so we cannot immediately instantiate the database
      // without any configuration, initSqlJs will fetch the wasm files directly from the same path as the js
      // see ../craco.config.js
      try {
        await initDB();
        await importDB(new URL('db/main.db', window.location.href));
        const db = getDB();
        setDB(db);
      } catch (err) {
        setError(err);
        window.console.error(err);
      }
    }

    fetchDB();
  }, []);

  return (
    <div className="App">
      <AppNavbar />
      <div className="content">
        {
          error ? <p>
            <strong>ERROR:</strong><br />
            {error.toString()}
          </p> : null
        }
        {
          !db
            ? (
              <div>
                <img src={logo} className="App-logo" alt="logo" />
                <p>Loading...</p>
              </div>
            )
            : (
              <Section>
                <Container>
                  {/* <Columns>
                    <Columns.Column is='3'>
                      Uhh
                    </Columns.Column>
                    <Columns.Column is='9'> */}
                  <MainPage db={db} />
                  {/* </Columns.Column>
                  </Columns> */}
                </Container>
              </Section>
            )
        }
      </div>
    </div>
  );
}

export default App;
