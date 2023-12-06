import React from "react";
import RatingChart from "./RatingChart";

class App extends React.Component {
  constructor(props) {
    super(props);

    this.handleMessage = this.handleMessage.bind(this);

    this.ws = new WebSocket("ws://localhost:8080/ws");

    this.ws.onopen = () => {
      this.setState({ connected: true });
    };
    this.ws.onerror = (e) => {
      console.log("ws error:", e);
    };
    this.ws.onmessage = e => {
      this.handleMessage(e);
    };
    this.ws.onclose = () => {
      this.setState({ connected: false });
    };
    this.ws.onopen = () => {
      this.setState({ connected: true });
    };

    this.state = {
      connected: false,
      chart_data: [["X", "Rating"]],
    };
  }
  chunk(arr, chunkSize) {
    const len = Math.ceil(arr.length / chunkSize)
    return Array.from({length: len}, () => arr.splice(0,chunkSize));
  }
  handleMessage(e) {
    var obj = JSON.parse(e.data);

    switch (obj.command) {
      case "add_rating":
        var arr = this.state.chart_data;
        obj.data.forEach(e => {
          arr.push([arr.length, e]);
        });
        this.setState({ chart_data: arr });
        break;
      case "info":
        this.setState({
          info: true, 
          player: obj.data.CurrentPlayer.User, 
          pgn: this.chunk(obj.data.Game.Chess.trim().split(" "), 3)
        })
        break;
      default:
        break;
    }
  }
  render() {
    return (
      <div>
        <div>
          {this.state.connected && (
            <RatingChart ws={this.ws} chart_data={this.state.chart_data} />
          )}
        </div>
        {this.state.info && (
        <div>
          <table>
            <tbody>
              <tr><td>Player</td><td>{this.state.player.Name}</td></tr>
              <tr><td>Room</td><td>#{this.state.player.RoomID}</td></tr>
              <tr><td>Rating</td><td>{this.state.player.Rating}</td></tr>
            </tbody>
          </table>
          <br/>
          <table>
            <tbody>
              {this.state.pgn.slice(-10).map(row => {
                return <tr><td>{row[0]}</td><td>{row[1]}</td><td>{row[2]}</td></tr>
              })}
            </tbody>
          </table>
        </div>
        )}
      </div>
    );
  }
}

export default App;
