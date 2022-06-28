import { useState } from 'react';
import './App.css';
import { Button, Table } from 'react-bootstrap';

interface User {
  name: string
  email: string
}

function App() {
  const [show, setShow] = useState(false)
  const [data, setData] = useState([] as User[])
  const listUsers = () => {
    console.log("fetch api")
    fetch("/users")
      .then((response) => response.json())
      .then((json) => {
        console.log(json);
        setData(json)
        setShow(true)
      });
  }
  let table = <div></div>
  if (show) {
    table = (
      <div className="AppInfo">
        <Table striped bordered hover variant="dark">
          <thead>
            <tr>
              <th>#</th>
              <th>Name</th>
              <th>Email</th>
            </tr>
          </thead>
          <tbody>
            {data.map((user, index) =>
              <tr key={index}>
                <th>{index}</th>
                <th>{user.name}</th>
                <th>{user.email}</th>
              </tr>
            )}
          </tbody>
        </Table>
      </div>
    )
  }
  return (
    <div className="App">
      <Button onClick={listUsers}>Click to show users</Button>
      {table}
    </div>
  );
}

export default App;
