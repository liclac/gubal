import dash
from dash.dependencies import Input, Output
import dash_core_components as dcc
import dash_html_components as html
import plotly.graph_objs as go
from flask_sqlalchemy import SQLAlchemy
import pandas as pd

app = dash.Dash()
app.server.config['SQLALCHEMY_DATABASE_URI'] = 'postgres:///gubal'
app.server.config['SQLALCHEMY_TRACK_MODIFICATIONS'] = False
db = SQLAlchemy(app.server)

# Load Bootstrap
app.css.append_css({
    "external_url": "https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css",
})
app.scripts.append_script({
    "external_url": "https://code.jquery.com/jquery-3.2.1.slim.min.js",
})
app.scripts.append_script({
    "external_url": "https://cdnjs.cloudflare.com/ajax/libs/popper.js/1.12.9/umd/popper.min.js",
})
app.scripts.append_script({
    "external_url": "https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/js/bootstrap.min.js",
})

def build_gender_chart(**kwargs):
    df = pd.read_sql('SELECT gender, COUNT(*) FROM characters GROUP BY gender ORDER BY gender DESC', db.engine, index_col='gender')
    return go.Pie(labels=df.index.tolist(), values=df['count'], **kwargs)

def build_race_chart(**kwargs):
    df = pd.read_sql('SELECT race, COUNT(*) FROM characters GROUP BY race ORDER BY race DESC', db.engine, index_col='race')
    return go.Pie(labels=df.index.tolist(), values=df['count'], **kwargs)

def build_layout():
    return html.Div([
        html.Div([
            dcc.Graph(
                id="gender-chart",
                figure=go.Figure(
                    data=[build_gender_chart(
                        textinfo='label+percent',
                        hole=0.5,
                        marker=go.Marker(
                            colors=['#00CCFF', '#FF0099'],
                            line=go.Line(color='#FFF', width=1),
                        ),
                    )],
                    layout=go.Layout(title="Gender"),
                ),
                className='col-sm-6',
            ),
            dcc.Graph(
                id="race-chart",
                figure=go.Figure(
                    data=[build_race_chart(
                        textinfo='label+percent',
                        hole=0.5,
                        marker=go.Marker(line=go.Line(color='#FFF', width=1)),
                    )],
                    layout=go.Layout(title="Race"),
                ),
                className='col-sm-6',
            ),
        ], className='row', style={'min-height': '500px'}),
    ], className='container')

app.layout = build_layout

if __name__ == '__main__':
    app.run_server(debug=True)
