import { Navbar } from "react-bulma-components";

import styles from './Navbar.module.scss';

const logoText = 'RumbleLog'

function AppNavbar() {
  return (
    <Navbar>
      <Navbar.Brand>
        <Navbar.Item
          href="/"
          className={[styles.layers, styles.glitch].join(' ')}
          data-text={logoText}>
          <span>{logoText}</span>
        </Navbar.Item>
        <Navbar.Burger />
      </Navbar.Brand>

      {/* <Navbar.Menu>
        <Navbar.Container align='left'>
          <Navbar.Item href="#">
            <Navbar.Link>
              First
            </Navbar.Link>
            <Navbar.Dropdown>
              <Navbar.Item href="#">
                Subitem 1
              </Navbar.Item>
              <Navbar.Item href="#">
                Subitem 2
              </Navbar.Item>
              <Navbar.Divider />
              <Navbar.Item href="#">
                After divider
              </Navbar.Item>
            </Navbar.Dropdown>
          </Navbar.Item>
          <Navbar.Item href="#">
            Second
          </Navbar.Item>
        </Navbar.Container>
        <Navbar.Container align="right">
          <Navbar.Item>
            <div className="buttons">
              <Button color="primary">
                <strong>Sign up</strong>
              </Button>
              <Button color="light">
                <strong>Log in</strong>
              </Button>
            </div>
          </Navbar.Item>
        </Navbar.Container>
      </Navbar.Menu> */}
    </Navbar >
  )
}

export default AppNavbar;
