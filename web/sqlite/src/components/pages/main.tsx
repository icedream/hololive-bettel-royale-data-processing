import SearchField from "../input/SearchField";
import type { Database } from '../../database';
import { useState } from "react";
import { getUsers, GetUsersRow } from "../../database/users.sql";
import { Icon, Menu, Tabs } from "react-bulma-components";
import { getInteractions, GetInteractionsRow } from "../../database/interactions.sql";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { faHand, faMouse, faUser } from "@fortawesome/free-solid-svg-icons";

function MainPage({
  db,
}: {
  db: Database,
}) {
  const [searchWorking, setSearchWorking] = useState<boolean>(false);
  const [usersRows, setUsersRows] = useState<GetUsersRow[] | null>(null);
  const [interactionsRows, setInteractionsRows] = useState<GetInteractionsRow[] | null>(null);

  return (
    <div>
      <Tabs>
        <Tabs.Tab active={true}>
          <Icon align="left">
            <FontAwesomeIcon icon={faUser} />
          </Icon>
          Users
        </Tabs.Tab>
        <Tabs.Tab>
          <Icon align="left">
            <FontAwesomeIcon icon={faMouse} />
          </Icon>
          Interactions
        </Tabs.Tab>
        <Tabs.Tab>
          <Icon align="left">
            <FontAwesomeIcon icon={faHand} />
          </Icon>
          Items
        </Tabs.Tab>
      </Tabs>
      <form>
        <SearchField
          onChange={(e) => {
            setSearchWorking(true);
            try {
              if (e.target.value.length === 0) {
                setUsersRows(null);
                setInteractionsRows(null);
                return;
              }

              const users = getUsers(db, {
                userName: null,
                userNameLike: `%${e.target.value}%`,
                offset: 0,
                maxCount: -1,
              });
              setUsersRows(users);

              const interactions = getInteractions(db, {
                itemName: null,
                userId: null,
                userName: e.target.value,
                userNameLike: null,
                messageText: null,
                // messageTextLike: `%${e.target.value}%`,
                messageTextLike: null,
                offset: 0,
                maxCount: -1,
              });
              setInteractionsRows(interactions);
            } finally {
              setSearchWorking(false);
            }
          }}
          working={searchWorking}
        />
        <Menu>
          {
            usersRows && usersRows.length > 0
              ? (
                <Menu.List title="Users">
                  {
                    usersRows?.map(r => <Menu.List.Item>{r.userName}</Menu.List.Item>)
                  }
                </Menu.List>
              )
              : ''
          }
          {
            interactionsRows && interactionsRows.length > 0
              ? (
                <Menu.List title="Interactions">
                  {
                    interactionsRows?.map(r => <Menu.List.Item>{r.text}</Menu.List.Item>)
                  }
                </Menu.List>
              )
              : ''
          }
        </Menu>
      </form>
    </div>
  );


}


export default MainPage;
