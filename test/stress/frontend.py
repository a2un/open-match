# Copyright 2019 Google LLC
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
 
from locust import HttpLocust, TaskSequence, task, seq_task
import random
import string
import json

ID_LEN = 6

class ClientBehavior(TaskSequence):

  def on_start(self):
    """ on_start is called when a Locust start before any task is scheduled """
    self.init()

  def init(self):
    # Placeholder for initialize future TLS materials and request generators
    random_generator = lambda len : ''.join(random.choices(string.ascii_uppercase + string.digits, k=len))
    self.ticket_generator = lambda : {
        "ticket": {
            "id": random_generator(ID_LEN),
            "properties": json.dumps({
                "level": random_generator(ID_LEN),
                "strength": random_generator(ID_LEN),
                "agility": random_generator(ID_LEN)
            })
        }
    }

    self.payload = {
        "endpoint": "/v1/frontend/tickets",
        "params": None,
        "body": None
    }

  @seq_task(1)
  @task(5)
  def create_ticket(self):
    self.payload["body"] = self.ticket_generator()

    method, endpoint, params, data, name = "POST", self.payload["endpoint"], None, json.dumps(self.payload["body"]), "Create: {}".format(self.payload["endpoint"])

    with self.client.request(method, endpoint, name=name, params=params, data=data, catch_response=True) as response:
      if response.content != b"{}":
        response.failure("Got {}, was expected {{}}".format(response.content))

  @seq_task(2)
  @task(5)
  def delete_ticket(self):
    method, endpoint, params, data, name = "DELETE", "{}/{}".format(self.payload["endpoint"], self.payload["body"]["ticket"]["id"]), None, None, "Delete: {}/[id]".format(self.payload["endpoint"])

    with self.client.request(method, endpoint, name=name, params=params, data=data, catch_response=True) as response:
      if response.content != b"{}":
        response.failure("Got {}, was expected {{}}".format(response.content))


class WebsiteUser(HttpLocust):
  task_set = ClientBehavior
  min_wait = 500
  max_wait = 1500
